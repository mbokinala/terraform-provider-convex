# Goal

Build a basic Terraform provider for Convex that imports an already-created Convex project and manages environment variables for its existing deployment.

## Cleaned-up Prompt

I want to build a minimal Terraform provider for Convex focused on one job: managing environment variables for a Convex project that already exists. The project and its deployment are created outside Terraform, and Convex assigns the deployment name, deployment ID, and related metadata when the project is created. Terraform should import that existing Convex project into state, expose the Convex-assigned identifiers as computed attributes, and then manage environment variables against the existing deployment one resource at a time.

## Objective

Deliver a first working provider that lets Terraform:

- authenticate to the Convex API
- import an existing Convex project that was created outside Terraform
- read the Convex-assigned deployment metadata for that project
- manage individual environment variables for that deployment
- read the current value of each managed environment variable

## Preferred User Experience

The first version should optimize for a resource-per-environment-variable workflow.

Users should look up an existing Convex deployment with a data source. The provider should treat deployment metadata such as deployment name and deployment ID as read-only values returned by Convex, not as user-managed inputs.

Users should be able to manage multiple variables with `for_each`, for example:

```hcl
locals {
  env_vars = {
    DATABASE_URL   = var.database_url
    API_SECRET     = var.api_secret
    FEATURE_FLAG_X = "true"
  }
}

data "convex_deployment" "prod" {
  name = "proficient-ptarmigan-541"
}

resource "convex_environment_variable" "this" {
  for_each      = local.env_vars
  deployment_id = data.convex_deployment.prod.id
  name          = each.key
  value         = each.value
}
```

They should also be able to manage a single variable directly:

```hcl
provider "convex" {
  team_access_token = var.convex_team_access_token
}

data "convex_deployment" "prod" {
  name = "proficient-ptarmigan-541"
}

resource "convex_environment_variable" "database_url" {
  deployment_id = data.convex_deployment.prod.id
  name          = "DATABASE_URL"
  value         = var.database_url
}
```

## Recommended First Scope

Keep the first version narrow. Start with:

- provider: `convex`
- data source: `convex_deployment`
- resource: `convex_environment_variable`

The `convex_deployment` data source should look up a deployment that already exists in Convex and expose read-only metadata returned by Convex, such as:

- project ID
- deployment ID
- deployment name

The resource should manage exactly one environment variable on one deployment:

- `deployment_id`
- `name`
- `value`

This keeps the Terraform model idiomatic, works naturally with `for_each`, and avoids coupling the full deployment environment into one large state object.

## Lookup Requirement

Because the project and deployment already exist, lookup is part of the initial milestone.

The provider should support reading an existing `convex_deployment` by deployment name and using its computed ID for downstream resources.

Importing existing environment variables is not required for V1. The initial workflow can assume Terraform looks up the existing deployment and then manages environment variables from that point forward.

## Success Criteria

The first version is successful if a user can:

1. configure the provider with credentials
2. look up an existing Convex deployment into `convex_deployment`
3. read the Convex-assigned deployment metadata from Terraform state
4. declare one or more env vars with `convex_environment_variable`
5. run `terraform plan` and see drift between Terraform and Convex
6. run `terraform apply` and have Convex updated to match Terraform

## Non-Goals For V1

Do not expand scope yet into:

- creating Convex projects or deployments
- choosing or customizing deployment IDs, deployment names, or other Convex-created metadata
- managing other Convex resources
- importing existing environment variables into Terraform state
- bulk full-deployment env var resources such as `convex_deployment_env_vars`
- complex secret workflows beyond what the existing env var API already supports

## Assumptions

- Convex already provides stable API endpoints for env var management
- the API can identify an existing project and its deployment unambiguously for import
- Convex returns deployment metadata such as deployment name and deployment ID after import/read
- the provider can read back current env vars after write operations

## Practical V1 Design Direction

Start with one provider and two resources:

- provider: `convex`
- data source: `convex_deployment`
- resource: `convex_environment_variable`

That gives users an idiomatic Terraform workflow around an already-created Convex deployment, keeps Convex-assigned deployment metadata authoritative, and leaves room to add more Convex resources later without redesigning the core env var interface.
