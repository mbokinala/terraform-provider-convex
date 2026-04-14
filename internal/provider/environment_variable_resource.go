package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/mbokinala/terraform-provider-convex/internal/client"
)

var (
	_ resource.Resource                = &environmentVariableResource{}
	_ resource.ResourceWithConfigure   = &environmentVariableResource{}
	_ resource.ResourceWithImportState = &environmentVariableResource{}
)

type environmentVariableResource struct {
	client *client.ConvexClient
}

type environmentVariableResourceModel struct {
	DeploymentID types.Int64  `tfsdk:"deployment_id"`
	Name         types.String `tfsdk:"name"`
	Value        types.String `tfsdk:"value"`
}

func NewEnvironmentVariableResource() resource.Resource {
	return &environmentVariableResource{}
}

func (r *environmentVariableResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environment_variable"
}

func (r *environmentVariableResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"deployment_id": schema.Int64Attribute{
				Required:    true,
				Description: "Convex deployment ID.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Environment variable name.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"value": schema.StringAttribute{
				Required:    true,
				Sensitive:   true,
				Description: "Environment variable value.",
			},
		},
	}
}

func (r *environmentVariableResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	convexClient, ok := req.ProviderData.(*client.ConvexClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Provider Data Type",
			fmt.Sprintf("Expected provider data to be *client.ConvexClient, got %T. This is a provider bug.", req.ProviderData),
		)
		return
	}

	r.client = convexClient
}

func (r *environmentVariableResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	convexClient, err := r.getClient()
	if err != nil {
		resp.Diagnostics.AddError("Missing Convex API Client", err.Error())
		return
	}

	var plan environmentVariableResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deploymentID, name, value, ok := validateEnvironmentVariableWriteModel(plan, &resp.Diagnostics, "create")
	if !ok {
		return
	}

	deployment, ok := resolveEnvironmentVariableDeployment(ctx, convexClient, deploymentID, name, &resp.Diagnostics, "create")
	if !ok {
		return
	}

	if err := convexClient.SetEnvironmentVariablesContext(ctx, deployment.Name, []client.EnvVarChange{{
		Name:  name,
		Value: &value,
	}}); err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Convex Environment Variable",
			fmt.Sprintf("Could not create environment variable %q on deployment %d (%q): %s", name, deploymentID, deployment.Name, err),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	readReq := resource.ReadRequest{
		State:    resp.State,
		Identity: resp.Identity,
		Private:  resp.Private,
	}
	readResp := &resource.ReadResponse{
		State:    resp.State,
		Identity: resp.Identity,
		Private:  resp.Private,
	}

	r.Read(ctx, readReq, readResp)
	resp.Diagnostics.Append(readResp.Diagnostics...)
	resp.State = readResp.State
	resp.Identity = readResp.Identity
	resp.Private = readResp.Private
}

func (r *environmentVariableResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	convexClient, err := r.getClient()
	if err != nil {
		resp.Diagnostics.AddError("Missing Convex API Client", err.Error())
		return
	}

	var state environmentVariableResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deploymentID, name, ok := validateEnvironmentVariableStateLookup(state, &resp.Diagnostics, "read")
	if !ok {
		return
	}

	deployment, ok := resolveEnvironmentVariableDeployment(ctx, convexClient, deploymentID, name, &resp.Diagnostics, "read")
	if !ok {
		return
	}

	variables, err := convexClient.ListEnvironmentVariablesContext(ctx, deployment.Name)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Convex Environment Variable",
			fmt.Sprintf("Could not list environment variables on deployment %d (%q) while reading %q: %s", deploymentID, deployment.Name, name, err),
		)
		return
	}

	for _, variable := range variables {
		if variable.Name != name {
			continue
		}

		state.DeploymentID = types.Int64Value(deploymentID)
		state.Name = types.StringValue(variable.Name)
		state.Value = types.StringValue(variable.Value)

		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		return
	}

	resp.State.RemoveResource(ctx)
}

func (r *environmentVariableResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	convexClient, err := r.getClient()
	if err != nil {
		resp.Diagnostics.AddError("Missing Convex API Client", err.Error())
		return
	}

	var plan environmentVariableResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deploymentID, name, value, ok := validateEnvironmentVariableWriteModel(plan, &resp.Diagnostics, "update")
	if !ok {
		return
	}

	deployment, ok := resolveEnvironmentVariableDeployment(ctx, convexClient, deploymentID, name, &resp.Diagnostics, "update")
	if !ok {
		return
	}

	if err := convexClient.SetEnvironmentVariablesContext(ctx, deployment.Name, []client.EnvVarChange{{
		Name:  name,
		Value: &value,
	}}); err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Convex Environment Variable",
			fmt.Sprintf("Could not update environment variable %q on deployment %d (%q): %s", name, deploymentID, deployment.Name, err),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	readReq := resource.ReadRequest{
		State:    resp.State,
		Identity: resp.Identity,
		Private:  resp.Private,
	}
	readResp := &resource.ReadResponse{
		State:    resp.State,
		Identity: resp.Identity,
		Private:  resp.Private,
	}

	r.Read(ctx, readReq, readResp)
	resp.Diagnostics.Append(readResp.Diagnostics...)
	resp.State = readResp.State
	resp.Identity = readResp.Identity
	resp.Private = readResp.Private
}

func (r *environmentVariableResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	convexClient, err := r.getClient()
	if err != nil {
		resp.Diagnostics.AddError("Missing Convex API Client", err.Error())
		return
	}

	var state environmentVariableResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deploymentID, name, ok := validateEnvironmentVariableStateLookup(state, &resp.Diagnostics, "delete")
	if !ok {
		return
	}

	deployment, ok := resolveEnvironmentVariableDeployment(ctx, convexClient, deploymentID, name, &resp.Diagnostics, "delete")
	if !ok {
		return
	}

	if err := convexClient.DeleteEnvironmentVariableContext(ctx, deployment.Name, name); err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Convex Environment Variable",
			fmt.Sprintf("Could not delete environment variable %q from deployment %d (%q): %s", name, deploymentID, deployment.Name, err),
		)
	}
}

func (r *environmentVariableResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importID := strings.TrimSpace(req.ID)
	if importID == "" {
		resp.Diagnostics.AddError(
			"Missing Import ID",
			"Import requires the ID in deployment_id:VAR_NAME format.",
		)
		return
	}

	deploymentIDText, name, found := strings.Cut(importID, ":")
	deploymentIDText = strings.TrimSpace(deploymentIDText)
	name = strings.TrimSpace(name)
	if !found || deploymentIDText == "" || name == "" {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Import ID %q is invalid. Use deployment_id:VAR_NAME.", importID),
		)
		return
	}

	deploymentID, err := strconv.ParseInt(deploymentIDText, 10, 64)
	if err != nil || deploymentID <= 0 {
		resp.Diagnostics.AddError(
			"Invalid Deployment ID",
			fmt.Sprintf("Import ID %q is invalid because %q is not a positive integer deployment ID.", importID, deploymentIDText),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("deployment_id"), deploymentID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), name)...)
}

func (r *environmentVariableResource) getClient() (*client.ConvexClient, error) {
	if r.client == nil {
		return nil, fmt.Errorf("the Convex API client is not configured. Ensure the provider is configured with a valid team_access_token before managing convex_environment_variable resources")
	}

	return r.client, nil
}

func validateEnvironmentVariableWriteModel(model environmentVariableResourceModel, diagnostics interface {
	AddError(summary string, detail string)
}, action string) (int64, string, string, bool) {
	deploymentID, ok := validateEnvironmentVariableDeploymentID(model.DeploymentID, diagnostics, action)
	if !ok {
		return 0, "", "", false
	}

	name, ok := validateEnvironmentVariableName(model.Name, diagnostics, action)
	if !ok {
		return 0, "", "", false
	}

	switch {
	case model.Value.IsUnknown():
		diagnostics.AddError(
			"Unknown Environment Variable Value",
			fmt.Sprintf("Cannot %s the Convex environment variable %q on deployment %d because the value is unknown.", action, name, deploymentID),
		)
		return 0, "", "", false
	case model.Value.IsNull():
		diagnostics.AddError(
			"Missing Environment Variable Value",
			fmt.Sprintf("Cannot %s the Convex environment variable %q on deployment %d because the value is null.", action, name, deploymentID),
		)
		return 0, "", "", false
	}

	return deploymentID, name, model.Value.ValueString(), true
}

func validateEnvironmentVariableStateLookup(model environmentVariableResourceModel, diagnostics interface {
	AddError(summary string, detail string)
}, action string) (int64, string, bool) {
	deploymentID, ok := validateEnvironmentVariableDeploymentID(model.DeploymentID, diagnostics, action)
	if !ok {
		return 0, "", false
	}

	name, ok := validateEnvironmentVariableName(model.Name, diagnostics, action)
	if !ok {
		return 0, "", false
	}

	return deploymentID, name, true
}

func validateEnvironmentVariableDeploymentID(id types.Int64, diagnostics interface {
	AddError(summary string, detail string)
}, action string) (int64, bool) {
	switch {
	case id.IsUnknown():
		diagnostics.AddError(
			"Unknown Deployment ID",
			fmt.Sprintf("Cannot %s the Convex environment variable because deployment_id is unknown.", action),
		)
		return 0, false
	case id.IsNull():
		diagnostics.AddError(
			"Missing Deployment ID",
			fmt.Sprintf("Cannot %s the Convex environment variable because deployment_id is null.", action),
		)
		return 0, false
	}

	value := id.ValueInt64()
	if value <= 0 {
		diagnostics.AddError(
			"Invalid Deployment ID",
			fmt.Sprintf("Cannot %s the Convex environment variable because deployment_id must be greater than zero.", action),
		)
		return 0, false
	}

	return value, true
}

func resolveEnvironmentVariableDeployment(
	ctx context.Context,
	convexClient *client.ConvexClient,
	deploymentID int64,
	name string,
	diagnostics interface {
		AddError(summary string, detail string)
	},
	action string,
) (client.Deployment, bool) {
	deployment, err := convexClient.GetDeploymentByIDContext(ctx, deploymentID)
	if err != nil {
		diagnostics.AddError(
			"Error Resolving Convex Deployment",
			fmt.Sprintf("Could not resolve deployment %d while attempting to %s environment variable %q: %s", deploymentID, action, name, err),
		)
		return client.Deployment{}, false
	}

	return deployment, true
}

func validateEnvironmentVariableName(name types.String, diagnostics interface {
	AddError(summary string, detail string)
}, action string) (string, bool) {
	switch {
	case name.IsUnknown():
		diagnostics.AddError(
			"Unknown Environment Variable Name",
			fmt.Sprintf("Cannot %s the Convex environment variable because the name is unknown.", action),
		)
		return "", false
	case name.IsNull():
		diagnostics.AddError(
			"Missing Environment Variable Name",
			fmt.Sprintf("Cannot %s the Convex environment variable because the name is null.", action),
		)
		return "", false
	}

	value := strings.TrimSpace(name.ValueString())
	if value == "" {
		diagnostics.AddError(
			"Invalid Environment Variable Name",
			fmt.Sprintf("Cannot %s the Convex environment variable because the name is empty.", action),
		)
		return "", false
	}

	return value, true
}
