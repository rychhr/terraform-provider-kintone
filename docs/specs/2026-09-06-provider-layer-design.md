# Provider Layer Design

**Status:** Accepted for v0.1.0. Local implementation verification is required; live acceptance results are recorded separately.

## Context and scope

The Terraform-independent client implements app creation, metadata reads, app listing, and general
settings updates. This design defines the provider configuration,
`kintone_app`, `data.kintone_app`, and `data.kintone_apps` behavior to build on that client.

Use the public names reserved in [CONTRIBUTING.md](../../CONTRIBUTING.md#naming-public-provider-interfaces)
and the measured/documented distinctions in [API constraints](../design/kintone-api-constraints.md).
Preview means draft settings; live means deployed settings. The app ID identifies the same app across
those environments. A revision describes API settings state, not Terraform resource identity.

## Setting ownership and omission

`name` is required. General settings are Optional + Computed by default, including optional nested
children where the API permits them. Explicit configuration manages the corresponding value. Omission,
including removing an existing setting from HCL, preserves the remote value rather than resetting it to
a provider-selected default. Live reads populate observed values in state.

Preserve the distinction between null, unknown, empty strings, and false. Unknown planned values must
not become zero-valued API updates. Before submitting a complete nested object, preserve omitted
children from the existing state or API as appropriate; the client merges nested updates with preview
settings. Compare configuration, plan, and state deliberately so an unrelated change does not send
every computed value back to the API.

Import identifies an app by its string app ID. An empty plan after import is required when the supplied
HCL agrees with existing values, including the required name. Import does not promise an empty plan
for contradictory configuration. Placement remains create-only: reject a requested space or thread
change with an attribute diagnostic rather than forcing replacement. This is a provider scope decision;
the separate [Move App to Space API](https://kintone.dev/en/docs/kintone/rest-api/apps/settings/move-app/)
supports ordinary-space moves, but is not implemented here.

## Creation failures and recovery

For v0.1.0, recovery from an interrupted or partially completed creation is manual. Preserve the known
app ID in state where possible and report the ID and failure stage in diagnostics. If the response did
not yield an ID, explain that manual inspection is required. Never invent an ID, report an unverified
deployment as successful, or revert drafts as cleanup.

Saving an ID is not an automatic recovery mechanism. Terraform marks a resource tainted when Create
returns an error and can plan recreation on the next run; see the
[framework Create caveats](https://developer.hashicorp.com/terraform/plugin/framework/resources/create#caveats).
The recovery documentation and tests must cover this next-plan behavior. Warn users to inspect the
app and state before retrying. Distinguish a never-deployed preview app from a deployed app whose
read-back failed; do not promise that normal live reads or import can recover both cases.

Automatic resume is outside v0.1.0. Delete removes state and warns about manual app cleanup; no physical
deletion is attempted. No schema attribute forces replacement. That rule does not suppress Terraform's
own taint-driven recreation behavior.

## Provider configuration and authentication

Reserve `base_url`, `username`, `password`, and `api_tokens` as the provider configuration attributes.
Explicit HCL values take precedence over environment variables. Unknown configuration must remain
unknown and must not silently fall back to environment credentials. Password and token values are
sensitive and must not appear in diagnostics.

When both complete password credentials and API tokens are supplied, password authentication wins,
matching the current client. Token-only configuration supports operations that accept tokens; operations
requiring password authentication within the provider's supported methods return diagnostics naming the
operation. The API itself also supports session and OAuth authentication for creation and listing.
Validate incomplete password credentials instead of silently changing authentication modes.

Use the established `KINTONE_BASE_URL`, `KINTONE_USERNAME`, and `KINTONE_PASSWORD` environment names.
`base_url`, `username`, and `password` are strings; `api_tokens` is a list of strings. The token
fallback is `KINTONE_API_TOKENS`, a comma-separated list with surrounding whitespace trimmed per entry,
limited to nine nonempty tokens. HCL tokens are used verbatim and validated by the client. Null HCL
attributes use the environment; unknown attributes do not. Explicit empty strings for the origin or
password credentials are invalid. An explicit empty token list suppresses token environment fallback;
complete password credentials are then needed.

## Data sources

`data.kintone_app` accepts an app ID and returns live app metadata and general settings. A missing app
is an error for this data source.

`data.kintone_apps` supports the client's ID, app-code, name, and space filters, reads every matching
page, and returns app metadata. It does not fetch general settings for each result. No matches produce
an empty collection; a failed page must not produce apparently successful partial results. Listing
requires password authentication through this provider.

The single data source takes required `id`. List filters are optional `ids`, `codes`, and `space_ids`
sets of strings (up to 100 members each) and optional `name` (up to 64 characters). The `apps` result is
a list of objects ordered by numeric app ID, with no synthetic data-source ID.

Metadata attributes are `id`, `code`, `name`, `description`, `space_id`, `thread_id`, `created_at`,
`creator`, `modified_at`, and `modifier`. Creator and modifier are objects containing `code` and `name`.
Placement is null outside a space. App IDs and revisions remain strings.

## Preview ownership

During Terraform management, operators must not leave unrelated, undeployed changes on the same app.
The client merges preview settings and deploys the app, so another actor's draft can be deployed along
with Terraform's change. Per-app locking serializes local client operations; it does not coordinate
writers in the UI or other processes.

The guarantee that changing one setting preserves the others assumes no unrelated preview changes or
concurrent external writer. Under that assumption, a settings-only change deploys exactly once, and an
unchanged configuration causes no settings write or deployment. Document the ownership assumption in
the resource documentation and examples.

## Required verification

- Exercise HCL/environment precedence, unknown configuration, both authentication modes, and
  operation-specific password requirements without exposing credentials.
- Check empty plans after fresh create, import with matching HCL, and general settings update.
- Cover omission after prior configuration, explicit false/empty values, and sibling-only nested
  updates through the provider layer, including null and unknown children.
- Cover failed creation before and after receiving an app ID, deployment failure, timeout, and failed
  live read-back. Use real Terraform against a local test server to verify partial state and the next
  plan's taint/recreation behavior.
- Verify create-only placement diagnostics, Delete's warning and state removal, single-source
  not-found errors, full pagination, empty lists, and failed-page handling.
- Count deployments for settings-only changes and assert no deployment for unchanged configuration.
- Run build, unit tests, and lint for implementation changes. Generate schema documentation and examples
  once schemas exist; compare them with the implemented schema.
- Run local tests with Terraform CLI 1.16.0 and the latest stable CLI; record the exact versions. The
  minimum supported CLI is 1.16.0; the plugin and Registry protocol remain 6.0.
- Run acceptance tests only with the complete dedicated development configuration and the repository's
  explicit infrastructure-change approval. Reuse a preselected published app for a token-only data-source
  read. The creation test checks number precision 12/3/HALF_EVEN, then updates only decimal places to 4
  alongside the description, preserving the sibling values and checking an empty plan. Expect one created
  app and two deployments per successful CLI-version run. Follow the
  [acceptance-test procedure](../../CONTRIBUTING.md#acceptance-tests) and record every created `tfacc-` app
  for manual cleanup.

## Remaining verification and design boundaries

The [title-field measurements](../design/kintone-api-constraints.md#7-open-questions) establish that AUTO
returns a non-empty selected field code for the tested app. On MANUAL-to-AUTO transitions, both omitting
the code and sending the previous manual code resulted in the automatically selected code. An explicit
empty code was rejected. Preserve this distinction when finalizing the title-field schema: observed
AUTO codes must not be assumed to match a configured manual code. The existing client completes nested
objects before sending them. Local fixtures verify sibling preservation; the expanded live acceptance
case for partial number-precision configuration remains unexecuted until a separately approved run.

## Schema and lifecycle details

- `title_field` and `number_precision` are single object attributes, Optional + Computed, with Optional
  + Computed children. `selection_mode` accepts AUTO or MANUAL. In AUTO, `field_code` must be omitted
  from HCL; its computed value is the API-selected code. In MANUAL, the code must be nonempty, either
  explicitly configured or preserved from an observed existing title field. When changing modes, do
  not pin an omitted code to its previous planned value. An explicit empty code is invalid.
- `total_digits`, `decimal_places`, and `first_month_of_fiscal_year` are integers, respectively within
  1–30, 0–10, and 1–12. The client continues to carry their wire values as strings.
- `space_id` and `thread_id` are Optional + Computed strings. Omission after import preserves observed
  placement. `thread_id` requires `space_id` when creating an app. IDs configured for placement or
  import are positive decimal strings without leading zeros, within the client's signed 64-bit range.
- v0.1.0 exposes no timeout attributes. The client uses its five-minute deploy/poll deadline and the
  caller's context; that deadline does not cover the entire create or update operation.
- Resource Read retains state and reports a diagnostic for all API read errors, including 404. A live
  read cannot establish whether a failed creation left a preview-only app. Operators must verify
  access, deployment status, and manual deletion before removing state; an error is not an automatic
  recreation signal. Data-source read failures remain diagnostics.

## Manual recovery

After a failed create, preserve the reported app ID and inspect the app in kintone before applying again.
Terraform may have retained tainted state, so a subsequent plan can propose replacement even though the
old app still exists. Never treat an empty state as proof that the remote creation did not happen.

If the intended app is deployed and should be retained, reconcile its settings manually, remove only the
failed Terraform binding if present, and import the existing app into the intended address. Review a new
plan against HCL matching the deployed values before applying. For a preview-only app, first inspect and
complete its deployment manually or arrange manual cleanup; normal live import is not a preview recovery
operation. If the app is unwanted, delete it manually and reconcile its Terraform binding before creating
another. For an interrupted update, inspect both preview and live settings and refresh before retrying.

These are operator recovery steps, not automatic provider actions. No recovery path silently discards
preview drafts. Unknown API behavior beyond the measured title cases remains explicitly unverified.
