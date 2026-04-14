package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/mbokinala/terraform-provider-convex/internal/client"
)

var (
	_ datasource.DataSource              = &deploymentDataSource{}
	_ datasource.DataSourceWithConfigure = &deploymentDataSource{}
)

type deploymentDataSource struct {
	client *client.ConvexClient
}

type deploymentDataSourceModel struct {
	Name           types.String `tfsdk:"name"`
	ID             types.Int64  `tfsdk:"id"`
	ProjectID      types.Int64  `tfsdk:"project_id"`
	DeploymentType types.String `tfsdk:"deployment_type"`
}

func NewDeploymentDataSource() datasource.DataSource {
	return &deploymentDataSource{}
}

func (d *deploymentDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_deployment"
}

func (d *deploymentDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasourceschema.Schema{
		Attributes: map[string]datasourceschema.Attribute{
			"name": datasourceschema.StringAttribute{
				Required:    true,
				Description: "Convex deployment name.",
			},
			"id": datasourceschema.Int64Attribute{
				Computed:    true,
				Description: "Convex deployment ID.",
			},
			"project_id": datasourceschema.Int64Attribute{
				Computed:    true,
				Description: "Convex project ID for the deployment.",
			},
			"deployment_type": datasourceschema.StringAttribute{
				Computed:    true,
				Description: "Convex deployment type.",
			},
		},
	}
}

func (d *deploymentDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	d.client = convexClient
}

func (d *deploymentDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	convexClient, err := d.getClient()
	if err != nil {
		resp.Diagnostics.AddError("Missing Convex API Client", err.Error())
		return
	}

	var config deploymentDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deploymentName, ok := validateDeploymentDataSourceName(config.Name, &resp.Diagnostics)
	if !ok {
		return
	}

	deployment, err := convexClient.GetDeploymentByNameContext(ctx, deploymentName)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Convex Deployment",
			fmt.Sprintf("Could not read deployment %q: %s", deploymentName, err),
		)
		return
	}

	state := deploymentDataSourceModel{
		Name:           types.StringValue(deployment.Name),
		ID:             types.Int64Value(deployment.ID),
		ProjectID:      types.Int64Value(deployment.ProjectID),
		DeploymentType: types.StringValue(deployment.DeploymentType),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (d *deploymentDataSource) getClient() (*client.ConvexClient, error) {
	if d.client == nil {
		return nil, fmt.Errorf("the Convex API client is not configured. Ensure the provider is configured with a valid team_access_token before using convex_deployment")
	}

	return d.client, nil
}

func validateDeploymentDataSourceName(value types.String, diagnostics interface {
	AddError(summary string, detail string)
}) (string, bool) {
	switch {
	case value.IsUnknown():
		diagnostics.AddError(
			"Unknown Deployment Name",
			"Cannot read the Convex deployment because name is unknown.",
		)
		return "", false
	case value.IsNull():
		diagnostics.AddError(
			"Missing Deployment Name",
			"Cannot read the Convex deployment because name is null.",
		)
		return "", false
	}

	deploymentName := strings.TrimSpace(value.ValueString())
	if deploymentName == "" {
		diagnostics.AddError(
			"Invalid Deployment Name",
			"Cannot read the Convex deployment because name is empty.",
		)
		return "", false
	}

	return deploymentName, true
}
