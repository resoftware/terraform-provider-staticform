# Terraform / OpenTofu Provider for StaticForm

Manage [StaticForm](https://staticform.app) forms, API keys, and integrations as
code. The provider works with both Terraform and OpenTofu (same plugin protocol)
and talks to the StaticForm REST API at `https://api.staticform.app`.

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
| Resource | `staticform_form` (fields + submit actions: email, webhook, Google Sheets, Notion) |
| Resource | `staticform_api_key` |
| Resource | `staticform_smtp_connection` |
| Resource | `staticform_email_domain` |
| Resource | `staticform_collaborator` |
| Data source | `staticform_form`, `staticform_forms` |
| Data source | `staticform_api_key_permissions` |

OAuth-backed connections (Google, Notion, Stripe) are created interactively in
the dashboard and referenced by ID inside submit actions; they are not managed as
resources.

## Development

```sh
make build       # compile
make test        # unit tests
make testacc     # acceptance tests (needs STATICFORM_API_KEY, use an sf_test_ key)
make docs        # regenerate docs/ from schema + examples (needs terraform on PATH)
make snapshot    # local GoReleaser build to validate the release pipeline
```

### Local install for manual testing

Point Terraform/OpenTofu at a locally built binary with a dev override in
`~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "resoftware/staticform" = "/path/to/your/GOBIN"
  }
  direct {}
}
```

Then `go install .` and run `terraform plan` against your config.

## Releasing

Releases are cut by pushing a `vX.Y.Z` tag. GitHub Actions runs GoReleaser, which
builds cross-platform binaries and a GPG-signed `SHA256SUMS` file for the
Terraform Registry. The repo needs `GPG_PRIVATE_KEY` and `PASSPHRASE` secrets, and
the matching public key registered with the Registry account.

## License

[MPL-2.0](LICENSE)
