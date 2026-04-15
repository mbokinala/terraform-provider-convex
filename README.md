# terraform-provider-convex

Terraform provider for Convex.

Today it is intentionally small: it supports looking up an existing Convex deployment and managing its environment variables. More resources will come later.

## Usage

Configure the provider with your Convex team access token:

```hcl
terraform {
  required_providers {
    convex = {
      source = "mbokinala/convex"
    }
  }
}

provider "convex" {
  team_access_token = var.convex_team_access_token
}

data "convex_deployment" "prod" {
  name = "your-deployment-name"
}

resource "convex_environment_variable" "example" {
  deployment_id = data.convex_deployment.prod.id
  name          = "EXAMPLE_VAR"
  value         = "example-value"
}

# Environment variables are non-sensitive by default. Set sensitive = true
# and use sensitive_value when Terraform should redact the value.
resource "convex_environment_variable" "secret" {
  deployment_id    = data.convex_deployment.prod.id
  name             = "SECRET_VAR"
  sensitive        = true
  sensitive_value  = var.secret_value
}
```

You can also supply the token with `CONVEX_TEAM_ACCESS_TOKEN`.
