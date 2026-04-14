package provider

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/mbokinala/terraform-provider-convex/internal/client"
)

var _ provider.Provider = &ConvexProvider{}

type ConvexProvider struct {
	version string
}

type ConvexProviderModel struct {
	TeamAccessToken types.String `tfsdk:"team_access_token"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &ConvexProvider{
			version: version,
		}
	}
}

func (p *ConvexProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "convex"
	resp.Version = p.version
}

func (p *ConvexProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"team_access_token": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Convex team access token. Can also be set via CONVEX_TEAM_ACCESS_TOKEN env var.",
			},
		},
	}
}

func (p *ConvexProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config ConvexProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if config.TeamAccessToken.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("team_access_token"),
			"Unknown Convex Team Access Token",
			"The provider cannot create the Convex API client because team_access_token is unknown. Set team_access_token to a known value before planning or applying, or omit it and use CONVEX_TEAM_ACCESS_TOKEN instead.",
		)
		return
	}

	teamAccessToken := strings.TrimSpace(os.Getenv("CONVEX_TEAM_ACCESS_TOKEN"))
	if !config.TeamAccessToken.IsNull() {
		configuredToken := strings.TrimSpace(config.TeamAccessToken.ValueString())
		if configuredToken != "" {
			teamAccessToken = configuredToken
		}
	}

	if teamAccessToken == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("team_access_token"),
			"Missing Convex Team Access Token",
			"Set the provider team_access_token attribute or the CONVEX_TEAM_ACCESS_TOKEN environment variable so the provider can authenticate to the Convex Management API.",
		)
		return
	}

	convexClient, err := client.NewConvexClient(teamAccessToken)
	if err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("team_access_token"),
			"Invalid Convex Team Access Token",
			fmt.Sprintf("The provider could not create the Convex API client: %s", err),
		)
		return
	}

	if _, err := convexClient.GetTokenDetailsContext(ctx); err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("team_access_token"),
			"Error Resolving Convex Team ID",
			fmt.Sprintf("The provider could not resolve the team ID from the provided team access token: %s", err),
		)
		return
	}

	resp.DataSourceData = convexClient
	resp.ResourceData = convexClient
}

func (p *ConvexProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewEnvironmentVariableResource,
	}
}

func (p *ConvexProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewDeploymentDataSource,
	}
}
