package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	managementAPIBaseURL         = "https://api.convex.dev/v1"
	deploymentAPIBaseURLTemplate = "https://%s.convex.cloud/api/v1"

	tokenDetailsPath             = "/token_details"
	listProjectsPathTemplate     = "/teams/%d/list_projects"
	getProjectBySlugPathTemplate = "/teams/%s/projects/%s"
	listDeploymentsPathTemplate  = "/projects/%d/list_deployments"

	listEnvironmentVariablesPath   = "/list_environment_variables"
	updateEnvironmentVariablesPath = "/update_environment_variables"
)

type ConvexClient struct {
	token      string
	teamID     int64
	httpClient *http.Client
}

type TokenDetails struct {
	TeamID int64
}

type Project struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Slug   string `json:"slug"`
	TeamID int64  `json:"teamId"`
}

type Deployment struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	DeploymentType string `json:"deploymentType"`
	IsDefault      bool   `json:"isDefault"`
	ProjectID      int64  `json:"projectId"`
}

type EnvironmentVariable struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type EnvVarChange struct {
	Name  string  `json:"name"`
	Value *string `json:"value"`
}

type tokenDetailsResponse struct {
	TeamID *int64 `json:"teamId"`
}

type updateEnvironmentVariablesRequest struct {
	Changes []EnvVarChange `json:"changes"`
}

func NewConvexClient(token string) (*ConvexClient, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, fmt.Errorf("team access token cannot be empty")
	}

	return &ConvexClient{
		token: token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

func (c *ConvexClient) Token() string {
	return c.token
}

func (c *ConvexClient) TeamID() int64 {
	return c.teamID
}

func (c *ConvexClient) GetTokenDetails() (TokenDetails, error) {
	return c.GetTokenDetailsContext(context.Background())
}

func (c *ConvexClient) GetTokenDetailsContext(ctx context.Context) (TokenDetails, error) {
	req, err := c.newManagementRequest(ctx, http.MethodGet, tokenDetailsPath, nil)
	if err != nil {
		return TokenDetails{}, err
	}

	var details tokenDetailsResponse
	if err := c.do(req, &details); err != nil {
		return TokenDetails{}, err
	}

	if details.TeamID == nil || *details.TeamID <= 0 {
		return TokenDetails{}, fmt.Errorf("token_details response did not include a teamId; ensure the provider is configured with a team access token")
	}

	c.teamID = *details.TeamID

	return TokenDetails{TeamID: *details.TeamID}, nil
}

func (c *ConvexClient) ListProjects(teamID int64) ([]Project, error) {
	return c.ListProjectsContext(context.Background(), teamID)
}

func (c *ConvexClient) ListProjectsContext(ctx context.Context, teamID int64) ([]Project, error) {
	if teamID <= 0 {
		return nil, fmt.Errorf("team ID must be greater than zero")
	}

	req, err := c.newManagementRequest(ctx, http.MethodGet, fmt.Sprintf(listProjectsPathTemplate, teamID), nil)
	if err != nil {
		return nil, err
	}

	var projects []Project
	if err := c.do(req, &projects); err != nil {
		return nil, err
	}

	return projects, nil
}

func (c *ConvexClient) GetProjectBySlug(teamIDOrSlug string, projectSlug string) (Project, error) {
	return c.GetProjectBySlugContext(context.Background(), teamIDOrSlug, projectSlug)
}

func (c *ConvexClient) GetProjectBySlugContext(ctx context.Context, teamIDOrSlug string, projectSlug string) (Project, error) {
	teamIDOrSlug = strings.TrimSpace(teamIDOrSlug)
	projectSlug = strings.TrimSpace(projectSlug)
	if teamIDOrSlug == "" {
		return Project{}, fmt.Errorf("team ID or slug cannot be empty")
	}
	if projectSlug == "" {
		return Project{}, fmt.Errorf("project slug cannot be empty")
	}

	path := fmt.Sprintf(
		getProjectBySlugPathTemplate,
		url.PathEscape(teamIDOrSlug),
		url.PathEscape(projectSlug),
	)

	req, err := c.newManagementRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return Project{}, err
	}

	var project Project
	if err := c.do(req, &project); err != nil {
		return Project{}, err
	}

	return project, nil
}

func (c *ConvexClient) ListDeployments(projectID int64) ([]Deployment, error) {
	return c.ListDeploymentsContext(context.Background(), projectID)
}

func (c *ConvexClient) ListDeploymentsContext(ctx context.Context, projectID int64) ([]Deployment, error) {
	if projectID <= 0 {
		return nil, fmt.Errorf("project ID must be greater than zero")
	}

	req, err := c.newManagementRequest(ctx, http.MethodGet, fmt.Sprintf(listDeploymentsPathTemplate, projectID), nil)
	if err != nil {
		return nil, err
	}

	var deployments []Deployment
	if err := c.do(req, &deployments); err != nil {
		return nil, err
	}

	return deployments, nil
}

func (c *ConvexClient) GetDeploymentByName(name string) (Deployment, error) {
	return c.GetDeploymentByNameContext(context.Background(), name)
}

func (c *ConvexClient) GetDeploymentByNameContext(ctx context.Context, name string) (Deployment, error) {
	deployments, err := c.listAllDeploymentsContext(ctx)
	if err != nil {
		return Deployment{}, err
	}

	return selectDeploymentByName(deployments, name)
}

func (c *ConvexClient) GetDeploymentByID(id int64) (Deployment, error) {
	return c.GetDeploymentByIDContext(context.Background(), id)
}

func (c *ConvexClient) GetDeploymentByIDContext(ctx context.Context, id int64) (Deployment, error) {
	deployments, err := c.listAllDeploymentsContext(ctx)
	if err != nil {
		return Deployment{}, err
	}

	return selectDeploymentByID(deployments, id)
}

func (c *ConvexClient) ListEnvironmentVariables(deploymentName string) ([]EnvironmentVariable, error) {
	return c.ListEnvironmentVariablesContext(context.Background(), deploymentName)
}

func (c *ConvexClient) ListEnvironmentVariablesContext(ctx context.Context, deploymentName string) ([]EnvironmentVariable, error) {
	req, err := c.newDeploymentRequest(ctx, http.MethodGet, deploymentName, listEnvironmentVariablesPath, nil)
	if err != nil {
		return nil, err
	}

	var payload json.RawMessage
	if err := c.do(req, &payload); err != nil {
		return nil, err
	}

	variables, err := decodeEnvironmentVariablesResponse(payload)
	if err != nil {
		return nil, fmt.Errorf("decode environment variables payload: %w", err)
	}

	return variables, nil
}

func (c *ConvexClient) SetEnvironmentVariables(deploymentName string, changes []EnvVarChange) error {
	return c.SetEnvironmentVariablesContext(context.Background(), deploymentName, changes)
}

func (c *ConvexClient) SetEnvironmentVariablesContext(ctx context.Context, deploymentName string, changes []EnvVarChange) error {
	if len(changes) == 0 {
		return nil
	}

	req, err := c.newDeploymentRequest(ctx, http.MethodPost, deploymentName, updateEnvironmentVariablesPath, updateEnvironmentVariablesRequest{
		Changes: changes,
	})
	if err != nil {
		return err
	}

	return c.do(req, nil)
}

func (c *ConvexClient) DeleteEnvironmentVariable(deploymentName string, name string) error {
	return c.DeleteEnvironmentVariableContext(context.Background(), deploymentName, name)
}

func (c *ConvexClient) DeleteEnvironmentVariableContext(ctx context.Context, deploymentName string, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("environment variable name cannot be empty")
	}

	return c.SetEnvironmentVariablesContext(ctx, deploymentName, []EnvVarChange{
		{
			Name:  name,
			Value: nil,
		},
	})
}

func (c *ConvexClient) newManagementRequest(ctx context.Context, method string, path string, body any) (*http.Request, error) {
	return c.newRequest(ctx, method, managementAPIBaseURL+path, "Bearer "+c.token, body)
}

func (c *ConvexClient) newDeploymentRequest(ctx context.Context, method string, deploymentName string, path string, body any) (*http.Request, error) {
	deploymentName = strings.TrimSpace(deploymentName)
	if deploymentName == "" {
		return nil, fmt.Errorf("deployment name cannot be empty")
	}

	baseURL := fmt.Sprintf(deploymentAPIBaseURLTemplate, deploymentName)
	return c.newRequest(ctx, method, baseURL+path, "Convex "+c.token, body)
}

func (c *ConvexClient) newRequest(ctx context.Context, method string, rawURL string, authorization string, body any) (*http.Request, error) {
	var requestBody io.Reader

	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encode %s request body for %q: %w", method, rawURL, err)
		}

		requestBody = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, rawURL, requestBody)
	if err != nil {
		return nil, fmt.Errorf("create %s request for %q: %w", method, rawURL, err)
	}

	req.Header.Set("Authorization", authorization)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return req, nil
}

func (c *ConvexClient) do(req *http.Request, out any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send %s request to %q: %w", req.Method, req.URL.String(), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		if readErr != nil {
			return fmt.Errorf("%s %q returned status %s and the error response could not be read: %w", req.Method, req.URL.String(), resp.Status, readErr)
		}

		detail := strings.TrimSpace(string(body))
		if detail != "" {
			return fmt.Errorf("%s %q returned status %s: %s", req.Method, req.URL.String(), resp.Status, detail)
		}

		return fmt.Errorf("%s %q returned status %s", req.Method, req.URL.String(), resp.Status)
	}

	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode %s response from %q: %w", req.Method, req.URL.String(), err)
	}

	return nil
}

func decodeEnvironmentVariablesResponse(payload json.RawMessage) ([]EnvironmentVariable, error) {
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, nil
	}

	if trimmed[0] == '[' {
		return decodeEnvironmentVariablesCollection(trimmed)
	}

	if trimmed[0] != '{' {
		return nil, fmt.Errorf("unsupported top-level JSON token %q", string(trimmed[:1]))
	}

	var wrapped struct {
		EnvironmentVariables json.RawMessage `json:"environmentVariables"`
	}
	if err := json.Unmarshal(trimmed, &wrapped); err != nil {
		return nil, fmt.Errorf("unmarshal wrapped response: %w", err)
	}

	if len(bytes.TrimSpace(wrapped.EnvironmentVariables)) == 0 {
		return nil, fmt.Errorf("wrapped response did not include environmentVariables")
	}

	return decodeEnvironmentVariablesCollection(wrapped.EnvironmentVariables)
}

func decodeEnvironmentVariablesCollection(payload json.RawMessage) ([]EnvironmentVariable, error) {
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, nil
	}

	switch trimmed[0] {
	case '[':
		var variables []EnvironmentVariable
		if err := json.Unmarshal(trimmed, &variables); err != nil {
			return nil, fmt.Errorf("unmarshal environment variables array: %w", err)
		}

		return variables, nil
	case '{':
		var valueByName map[string]string
		if err := json.Unmarshal(trimmed, &valueByName); err == nil {
			names := make([]string, 0, len(valueByName))
			for name := range valueByName {
				names = append(names, name)
			}
			sort.Strings(names)

			variables := make([]EnvironmentVariable, 0, len(names))
			for _, name := range names {
				variables = append(variables, EnvironmentVariable{
					Name:  name,
					Value: valueByName[name],
				})
			}

			return variables, nil
		}

		return nil, fmt.Errorf("unsupported environment variables object shape")
	default:
		return nil, fmt.Errorf("unsupported environment variables JSON token %q", string(trimmed[:1]))
	}
}

func (c *ConvexClient) listAllDeploymentsContext(ctx context.Context) ([]Deployment, error) {
	if c.teamID <= 0 {
		return nil, fmt.Errorf("team ID must be resolved before listing deployments")
	}

	projects, err := c.ListProjectsContext(ctx, c.teamID)
	if err != nil {
		return nil, fmt.Errorf("list projects for team %d: %w", c.teamID, err)
	}

	deployments := make([]Deployment, 0)
	for _, project := range projects {
		projectDeployments, err := c.ListDeploymentsContext(ctx, project.ID)
		if err != nil {
			return nil, fmt.Errorf("list deployments for project %d: %w", project.ID, err)
		}

		deployments = append(deployments, projectDeployments...)
	}

	return deployments, nil
}

func selectDeploymentByName(deployments []Deployment, name string) (Deployment, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Deployment{}, fmt.Errorf("deployment name cannot be empty")
	}

	matches := make([]Deployment, 0, 1)
	for _, deployment := range deployments {
		if deployment.Name == name {
			matches = append(matches, deployment)
		}
	}

	return selectSingleDeployment(matches, fmt.Sprintf("name %q", name))
}

func selectDeploymentByID(deployments []Deployment, id int64) (Deployment, error) {
	if id <= 0 {
		return Deployment{}, fmt.Errorf("deployment ID must be greater than zero")
	}

	matches := make([]Deployment, 0, 1)
	for _, deployment := range deployments {
		if deployment.ID == id {
			matches = append(matches, deployment)
		}
	}

	return selectSingleDeployment(matches, fmt.Sprintf("ID %d", id))
}

func selectSingleDeployment(matches []Deployment, description string) (Deployment, error) {
	switch len(matches) {
	case 0:
		return Deployment{}, fmt.Errorf("no deployment matched %s", description)
	case 1:
		return matches[0], nil
	default:
		return Deployment{}, fmt.Errorf("multiple deployments matched %s", description)
	}
}
