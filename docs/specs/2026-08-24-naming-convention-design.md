# Provider Naming Convention Design

**Status:** Accepted for v0.1.0 schema design

**Issue:** #5

**Reference:** [HashiCorp provider naming conventions](https://developer.hashicorp.com/terraform/plugin/best-practices/naming)

## Context

Terraform resource types, data-source types, and attributes are public interfaces. Once published to the
Registry, a rename can break users' HCL configurations and state. The provider is still pre-release, so
this is the point to decide durable names before the v0.1.0 schema work in Issue #11 begins.

The convention is deliberately kept in `CONTRIBUTING.md`, where contributors can apply it while reviewing
new public interfaces. This design records the reasoning, scope decisions, and rejected alternatives that
would make the contributor guidance harder to use.

## Decision

Public provider names use English lowercase `snake_case`. Acronyms are ordinary lowercase words, including
`api`, `acl`, `id`, and `url`. Managed payloads use nouns rather than API operation or endpoint names.

Resource and data-source type names carry the provider prefix `kintone_`. The remaining name describes the
managed or returned payload:

- A resource for one object is singular.
- A singleton resource for a collection-wide payload is plural, even though the settings surface exists at
  most once per app.
- A data source for one object is singular; one that returns a list is plural.
- A qualified concept puts its direct target first: `app_acl`, `record_acl`, and `field_acl`.

Attributes are singular for one value and plural for a collection. Reference identifiers use
`<object>_id`; boolean state is affirmative and ends in `*_enabled`.

These rules follow HashiCorp's provider naming guidance while resolving the choices particular to kintone
settings resources.

## Resource ownership and singleton boundaries

`kintone_app` owns app identity, create-only placement, and general settings. Keeping those related app
concerns together gives the minimal core one authoritative app resource.

Other settings surfaces that are independently managed and exist at most once per app are standalone
singleton resources. Their import ID is the app ID. Their name reflects the payload's cardinality: a
resource managing a complete collection uses a plural name, while one managing a single settings object
uses a singular name.

An API endpoint is an implementation detail, not a Terraform resource boundary. A separate endpoint alone
does not justify another resource; the settings surface must have an independent management boundary. This
prevents endpoint structure from creating an arbitrary public interface.

## Applying the rules

The convention classifies representative surfaces without case-by-case exceptions:

| Surface | Classification | Result under the convention |
| --- | --- | --- |
| One app | Single entity | `kintone_app` |
| All form fields for one app | Collection-wide singleton | `kintone_form_fields` |
| App, record, and field permissions | Target-qualified ACLs | `kintone_app_acl`, `kintone_record_acl`, `kintone_field_acl` |
| App identity, placement, and general settings | App-owned general settings | `kintone_app` |

Only the v0.1.0 names listed below are reserved by this design. The form-fields and ACL type names
illustrate the rules for later roadmap work; they are not reserved interfaces and must be confirmed in
their own designs.

## v0.1.0 reservations

Issue #11 must use these names for the initial public app schema:

| Area | Reserved names |
| --- | --- |
| Managed resource | `kintone_app` |
| Single and list data sources | `kintone_app`, `kintone_apps` |
| Identity and lifecycle | `id`, `name`, `description`, `space_id`, `thread_id`, `revision` |
| General settings | `theme`, `title_field`, `number_precision`, `first_month_of_fiscal_year` |
| `title_field` children | `selection_mode`, `field_code` |
| `number_precision` children | `total_digits`, `decimal_places`, `rounding_mode` |
| Feature toggles | `thumbnails_enabled`, `bulk_deletion_enabled`, `comments_enabled`, `record_duplication_enabled`, `inline_record_editing_enabled` |

Provider authentication attributes and data-source output attributes remain open for Issue #11. That issue
must select them using this convention once their schemas and semantics are defined.

## Rejected alternatives

### Mirror kintone API casing or endpoint boundaries

The API's casing and endpoint layout are transport details. Exposing them directly would make Terraform
names inconsistent and would couple the public interface to API implementation structure.

### Make every singleton name singular

Singleton describes the number of settings surfaces per app, not the number of items in a payload. Naming a
complete collection singularly would hide its cardinality from configuration authors.

### Embed every settings surface in `kintone_app`

The app resource must own app identity, create-only placement, and general settings, but unrelated settings
surfaces can have independent management lifecycles. Embedding all of them would make one resource a
catch-all and prevent standalone singleton resources where their boundaries are warranted.

### Reserve later-roadmap names before their designs exist

Premature reservations would lock public interfaces before their payloads and lifecycles are understood.
Examples in this design demonstrate the convention only; each later minor release decides and reserves its
own names through design work.

## Consequences

Contributors apply the concise rules in `CONTRIBUTING.md` before adding or reviewing a public schema name.
The design leaves only the v0.1.0 names above reserved, allowing later releases to evolve from a stable
convention without guessing at their interfaces in advance.
