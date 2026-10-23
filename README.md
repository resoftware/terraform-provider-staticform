# StaticForm Provider for Terraform & OpenTofu

Manage [StaticForm](https://staticform.app) forms, API keys, and integrations as
code. Published to both the [Terraform Registry](https://registry.terraform.io)
and the [OpenTofu Registry](https://search.opentofu.org) from the same release
(identical plugin protocol). Talks to the StaticForm REST API at
`https://api.staticform.app/api/v1`.

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

## Sensitive fields

Requires the current StaticForm API. Sensitivity is server policy, separate from Terraform's
`sensitive` output-redaction keyword. The same configuration works with Terraform and OpenTofu:

```hcl
resource "staticform_form" "protected" {
  name                     = "Protected contact"
  confirm_sensitive_fields = true

  field {
    name      = "bsn"
    type      = "Text"
    sensitive = true
  }

  submit_action {
    type             = "SendEmailAction"
    name             = "Approved notification"
    recipient        = "authorized@example.com"
    body_template    = "Reference: {{form.bsn}}"
    sensitive_fields = ["bsn"]
  }
}
```

Sensitive fields stay encrypted and masked on ordinary reads. An action receives originals only
for its explicit `sensitive_fields` selection. The API requires `confirm_sensitive_fields = true`
for new approvals, destination/configuration changes and disabling protection for future submissions.
No password is required. Terraform/OpenTofu also emits a sharing warning. Review each plan before
applying: leaving the confirmation flag true acknowledges future applies too.

Omitting `field.sensitive` or `submit_action.sensitive_fields` preserves the server value across
refresh/import and unrelated updates. For new fields/actions, the defaults are false/no approval.
Use explicit `false` to stop protection for future intake, or an explicit empty set to revoke sharing.
Historical protected values remain encrypted. File fields cannot be marked sensitive.

The form data source exposes sensitive field names as `sensitive_fields`. Submission and execution-log
data sources keep the API's masked values; this provider never calls the sensitive-value reveal endpoint
or stores originals in state. Assigning read-sensitive permission to the provider key does not change that.

## Authentication

Create an API key under **Settings → API Keys** in the dashboard and pass it via
the `api_key` provider argument or the `STATICFORM_API_KEY` environment variable.
Use an `sf_test_` key against a test environment by also setting `base_url`
(or `STATICFORM_BASE_URL`).

API-key management uses the `/api/v1/api-keys` API. Creating a key requires the account's password or a
fresh, account-bound reauthentication proof. Set optional sensitive `key_creation_password` /
`key_creation_reauth_token`, or `STATICFORM_KEY_CREATION_PASSWORD` / `STATICFORM_KEY_CREATION_REAUTH_TOKEN`.
Only key creation sends these values. Accounts requiring another factor need a fresh reauthentication proof;
expired or consumed proofs must be renewed. Other resources, key reads, updates and deletion need no
creation proof, so existing form-only automation is unaffected.

Existing key resource IDs and saved plaintext survive refresh/import. Imports cannot recover a key's
plaintext. Paginated listing is followed across every page. Permissions use `User.*` names, as listed by the
`staticform_api_key_permissions` data source and used exactly as written. Historical product names (for example
`Form.Update` or `Integrations.ManageNotionConnections`) are translated to every per-endpoint permission that
replaced them, retaining their Terraform representation while the effective scope matches. Omitted permissions
inherit the owner's delegable permissions. Explicit empty permission sets are rejected because legacy updates
meant deny-all while empty scopes mean inheritance; revoke the resource to remove access. If creation succeeds
but metadata retrieval fails, the returned ID and plaintext remain in state for recovery.

An API key can only create, rotate, re-scope or delete keys whose permissions and form scopes it holds itself.
A provider key that is restricted to some permissions or forms must therefore give every managed key explicit
`permissions` (and `form_ids` within its own), because an omitted set inherits all of the owner's permissions.

### Permissions the provider key needs

Every endpoint has its own permission. When a request is refused for a missing permission, the error names it.
A key that only manages some resources needs only the permissions of those resources:

| Resource or data source | Permissions |
|---|---|
| `staticform_form` | `User.Forms.Create`, `User.Forms.GetById`, `User.Forms.Update`, `User.Forms.Delete`, `User.Forms.UpdateTags`; `User.Forms.UploadEmailAttachment` for `static_attachment` files uploaded from a `source` path |
| `staticform_forms` | `User.Forms.ListMy` |
| `staticform_form` (data source) | `User.Forms.GetById` |
| `staticform_submissions` | `User.Submissions.ListForForm` |
| `staticform_execution_logs` | `User.ExecutionLogs.ListAll` |
| `staticform_api_key` | `User.ApiKeys.List`, `User.ApiKeys.Create`, `User.ApiKeys.UpdatePermissions`, `User.ApiKeys.Delete`, plus every permission and form scope granted to the managed key |
| `staticform_api_key_permissions` | `User.Account.GetPermissions` |
| `staticform_smtp_connection` | `User.Integrations.ManageSmtpConnections`, `User.Integrations.ListSmtpConnections`, `User.Integrations.DeleteSmtpConnection` |
| `staticform_email_domain` | `User.Integrations.ManageEmailDomains`, `User.Integrations.ListEmailDomains`, `User.Integrations.DeleteEmailDomain` |
| `staticform_notion_connection` | `User.Integrations.ManageNotionConnections`, `User.Integrations.ListNotionConnections`, `User.Integrations.DeleteNotionConnection` |
| `staticform_stripe_connection` | `User.Integrations.ManageStripeConnections`, `User.Integrations.ListStripeConnections`, `User.Integrations.SetStripeConnectionWebhookSecret`, `User.Integrations.DisconnectStripeConnection` |
| `staticform_google_connections`, `staticform_notion_connections`, `staticform_stripe_connections` | `User.Integrations.ListGoogleConnections`, `User.Integrations.ListNotionConnections`, `User.Integrations.ListStripeConnections` |
| `staticform_collaborator` | `User.Collaboration.ManageCollaborators`, `User.Collaboration.ListCollaborators`, `User.Collaboration.ListInvitations`, `User.Collaboration.UpdateCollaborator`, `User.Collaboration.RemoveCollaborator`, `User.Collaboration.CancelInvitation` |
| `staticform_user_settings` | `User.Settings.Get`, `User.Settings.Update` |
| `staticform_notification_preferences` (resource) | `User.Notifications.GetPreferences`, `User.Notifications.UpdatePreferences` |
| `staticform_notification_preferences` (data source) | `User.Notifications.GetPreferences` |
| `staticform_subscription` | `User.Billing.Get` |
| `staticform_user` | `User.Account.Get`, `User.Billing.GetEntitlements` |

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
| Resource | `staticform_notification_preferences` |
| Data source | `staticform_form`, `staticform_forms`, `staticform_submissions` |
| Data source | `staticform_api_key_permissions` |
| Data source | `staticform_google_connections`, `staticform_notion_connections`, `staticform_stripe_connections` |
| Data source | `staticform_subscription`, `staticform_user`, `staticform_execution_logs`, `staticform_notification_preferences` |

Google connections use OAuth, so they are created interactively in the dashboard
and referenced by ID (look them up with the `staticform_google_connections` data
source). Notion and Stripe connections are token/key-based and **are** managed as
resources.

## Upgrading

The provider targets the single `/api/v1` API of the current StaticForm release. Compared with earlier
provider versions:

- `staticform_subscription` is read from `/api/v1/billing/me`. `tier`, `tier_name`, `price_in_cents` and
  `billing_interval` are gone. Use `status`, `interval`, `unit_price_amount`, `unit_price_currency`,
  `stripe_product_id`, `stripe_price_id`, `subscription_id`, `current_period_start`, `current_period_end`,
  `trial_end`, `canceled_at`, `has_billing_account`, `stripe_customer_id` and `entitlements`.
- `staticform_user_settings` only manages `submission_overview_email_frequency`. The attributes
  `submit_action_failure_notification_enabled`, `email_quota_warning_enabled` and `storage_warning_enabled` were
  removed from the API. They are per-event notification preferences now: manage them with the new
  `staticform_notification_preferences` resource under the event keys `product.SubmitActionFailed`,
  `product.EmailQuotaWarning` and `product.StorageWarning`, each with an `in_app` and an `email` choice (the old
  toggles controlled email only, so set `email`). The fourth removed attribute, email overage, no longer exists
  and has no replacement.
- `staticform_notification_preferences` is a singleton for the signed-in user. Only the events listed in
  `events` are managed, global attributes you omit keep their server value, and `quiet_hours` omitted means no
  quiet hours. Destroying it removes it from state and leaves the server preferences unchanged.
- `staticform_form` models `payment_required_condition` (a `payment` sub-block using the same condition syntax as
  `run_condition`) and an optional `condition` block on every `fixed_rule`, `field_amount_rule` and `line_item`.
  The API replaces a form's payment settings on every update, so the provider keeps what it does not manage:
  with `payment_required_condition` omitted, the condition the API holds (for example one set in the dashboard)
  is sent back unchanged and is not tracked in state; an empty block (`payment_required_condition {}`) removes
  it. Payment settings that are disabled in the dashboard are likewise kept when the configuration has no
  `payment` block. Conditions set in the dashboard with several groups are preserved when the block is omitted,
  but the provider models a single group, so a configured block replaces them with one group.
- `staticform_user` derives the `can_use_*` flags from the account's entitlements and `is_admin` from the
  superadmin flag.
- Permission names changed. Historical product names are translated automatically, including the per-endpoint
  permissions that replaced `Integrations.Manage*`, `Form.Update` and similar. A `User.*` name in your
  configuration is used exactly as written, so a key that held `User.Integrations.ManageNotionConnections`
  must now list `User.Integrations.ListNotionConnections` and `User.Integrations.DeleteNotionConnection` too.
- Errors are reported from the API's problem responses: title, detail, `errorCode` and per-field validation
  codes, and a refused request names the missing permission.

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
registries; both verify with the same GPG public key. The protocol manifest must be included in
both `release.extra_files` and `checksum.extra_files` with the same versioned filename.
Terraform Registry rejects a release when the signed checksum file omits the manifest.
For the `resoftware` namespace, resync missing versions in HCP Terraform under
**Registry > Public namespaces > resoftware > Providers > asset options > Resync provider**.

- **Terraform Registry**: sign in at [registry.terraform.io](https://registry.terraform.io),
  add the GPG public key under the `resoftware` organization, then
  **Publish → Provider**. Subsequent tagged releases are indexed automatically.
- **OpenTofu Registry**: one-time submission to the
  [`opentofu/registry`](https://github.com/opentofu/registry) repo (add the
  `resoftware` namespace + GPG public key); tagged releases are indexed thereafter.

## License

[MPL-2.0](LICENSE)
