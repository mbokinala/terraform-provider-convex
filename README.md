# Convex Terraform Provider

Terraform provider for Convex.

Terraform Registry: [mbokinala/convex](https://registry.terraform.io/providers/mbokinala/convex/)

Today it is intentionally small: it supports looking up an existing Convex deployment and managing its environment variables. More resources will come later.

## Usage

Install and reference the provider from the [Terraform Registry](https://registry.terraform.io/providers/mbokinala/convex/).

Configure the provider with your Convex team access token, and manage environment variables from terraform:

```hcl
terraform {
  required_providers {
    convex = {
      source = "mbokinala/convex"
    }
  }
}

variable "convex_team_access_token" {
  type      = string
  sensitive = true
}

provider "convex" {
  team_access_token = var.convex_team_access_token
}

data "convex_deployment" "prod" {
  name = "your-deployment-name" # e.g. nonchalant-tortoise-123
}

# Non-sensitive environment variable
resource "convex_environment_variable" "api_base_url" {
  deployment_id = data.convex_deployment.prod.id
  name          = "API_BASE_URL"
  value         = var.api_base_url
}

# Sensitive environment variable
resource "convex_environment_variable" "stripe_secret_key" {
  deployment_id   = data.convex_deployment.prod.id
  name            = "STRIPE_SECRET_KEY"
  sensitive       = true
  sensitive_value = var.stripe_secret_key
}
```

You can also supply the token by setting the `CONVEX_TEAM_ACCESS_TOKEN` environment variable.