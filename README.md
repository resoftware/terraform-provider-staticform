# StaticForm Provider for Terraform & OpenTofu

Manage [StaticForm](https://staticform.app) forms, API keys, and integrations as
code. Published to both the [Terraform Registry](https://registry.terraform.io)
and the [OpenTofu Registry](https://search.opentofu.org) from the same release
(identical plugin protocol). Talks to the StaticForm REST API at
`https://api.staticform.app`.

## Usage

```hcl
terraform {
  required_providers {
    staticform = {
      source = "resoftware/staticform"
    }
  }
}

provider "staticform" {
  # Or set STATICFORM_API_KEY in the environment.
  api_key = var.staticform_api_key
}

resource "staticform_form" "contact" {
  name = "Contact form"

  field {
    name     = "email"
    type     = "Text"
    rule     = "Email"
    required = true
  }

  submit_action {
    type          = "SendEmailAction"
    name          = "Notify team"
    recipient     = "team@example.com"
    subject       = "New message"
    body_template = "{{form.email}}"
  }
}
```

## Authentication

Create an API key under **Settings → API Keys** in the dashboard and pass it via
the `api_key` provider argument or the `STATICFORM_API_KEY` environment variable.
Use an `sf_test_` key against a test environment by also setting `base_url`
(or `STATICFORM_BASE_URL`).

## Resources & data sources

| Type | Name |
|------|------|
| Resource | `staticform_form` (fields; submit actions for email, webhook, Google Sheets, Notion; redirect; payment; tags; email attachments) |
| Resource | `staticform_api_key` |
| Resource | `staticform_smtp_connection` |
| Resource | `staticform_email_domain` |
| Resource | `staticform_notion_connection` |
| Resource | `staticform_stripe_connection` |
| Resource | `staticform_collaborator` |
| Resource | `staticform_user_settings` |
| Data source | `staticform_form`, `staticform_forms`, `staticform_submissions` |
| Data source | `staticform_api_key_permissions` |
| Data source | `staticform_google_connections`, `staticform_notion_connections`, `staticform_stripe_connections` |
| Data source | `staticform_subscription`, `staticform_user`, `staticform_execution_logs` |

Google connections use OAuth, so they are created interactively in the dashboard
and referenced by ID (look them up with the `staticform_google_connections` data
source). Notion and Stripe connections are token/key-based and **are** managed as
resources.

## Development

```sh
make build       # compile
make test        # unit tests
make testacc     # acceptance tests (needs STATICFORM_API_KEY, use an sf_test_ key)
make docs        # regenerate docs/ from schema + examples (needs the tofu or terraform CLI on PATH)
make snapshot    # local GoReleaser build to validate the release pipeline
```

### Local install for manual testing

Point Terraform/OpenTofu at a locally built binary with a dev override in
`~/.terraformrc` (Terraform) or `~/.tofurc` (OpenTofu):

```hcl
provider_installation {
  dev_overrides {
    "resoftware/staticform" = "/path/to/your/GOBIN"
  }
  direct {}
}
```

Then `go install .` and run `terraform plan` / `tofu plan` against your config.

## Releasing

Releases are cut by pushing a `vX.Y.Z` tag. GitHub Actions runs GoReleaser, which
builds cross-platform binaries and a GPG-signed `SHA256SUMS` file. The repo needs
`GPG_PRIVATE_KEY` and `PASSPHRASE` secrets. The same release artifacts feed both
registries; both verify with the same GPG public key.

- **Terraform Registry** — sign in at [registry.terraform.io](https://registry.terraform.io),
  add the GPG public key under the `resoftware` organization, then
  **Publish → Provider**. Subsequent tagged releases are indexed automatically.
- **OpenTofu Registry** — one-time submission to the
  [`opentofu/registry`](https://github.com/opentofu/registry) repo (add the
  `resoftware` namespace + GPG public key); tagged releases are indexed thereafter.

## License

[MPL-2.0](LICENSE)
