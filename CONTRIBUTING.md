# Contributing to the Terraform Provider for kintone

Issues, pull requests, commit messages, documentation, and diagnostics are written in English. Read
[AGENTS.md](AGENTS.md) for the repository workflow, testing requirements, API constraints, and release
rules.

## Naming public provider interfaces

Resource types, data-source types, and schema attributes are public Terraform interfaces. After a name is
published, changing it breaks existing configurations and state, so choose names with these rules before
adding a schema. The design rationale and rejected alternatives are in
[the naming-convention design](docs/specs/2026-08-24-naming-convention-design.md).

### General form

- Use English, lowercase `snake_case` names.
- Write acronyms as lowercase words: `api`, `acl`, `id`, and `url`.
- Use nouns for managed payloads. Do not name a resource after an API operation or endpoint.
- Qualify a concept by placing its direct target first: `app_acl`, `record_acl`, and `field_acl`.

### Resource and data-source types

- Name a resource for one managed object in the singular. For example, the app resource is
  `kintone_app`.
- Name a singleton resource that manages a collection-wide payload in the plural. For example,
  `kintone_form_fields` would manage the complete set of fields for one app, not one field. A singleton
  means that the settings surface exists at most once per app; it does not make the managed payload
  singular.
- Name a data source for one object in the singular and a data source that returns a list in the plural.
  For example, use `data.kintone_app` and `data.kintone_apps`.
- The `kintone_` provider prefix is part of every public resource and data-source type. The remainder of
  the type name follows the rules above.

### Attribute names

- Use singular names for a single value and plural names for a collection.
- Name a reference identifier `<object>_id`, such as `space_id` or `thread_id`.
- Name an affirmative boolean state `*_enabled`, such as `comments_enabled`. Do not use negative forms
  such as `comments_disabled`.

### Resource boundaries

`kintone_app` owns app identity, create-only placement, and the app's general settings. Do not split one of
those concerns into another resource merely because the kintone API exposes a different endpoint.

An independently managed settings surface that exists at most once per app is a standalone singleton
resource. Its import ID is the app ID. Give a settings surface its own resource only when it has an
independent management boundary; a separate API endpoint alone is not sufficient.

Later-roadmap names in this document are illustrative only. They are not reserved public interfaces until
their own design work explicitly reserves them.

## v0.1.0 names reserved for Issue #11

Issue #11 must use the following public names for the v0.1.0 app schema:

- Managed resource: `kintone_app`
- Single data source: `kintone_app`
- List data source: `kintone_apps`
- Identity and lifecycle attributes: `id`, `name`, `description`, `space_id`, `thread_id`, `revision`
- General settings: `theme`, `title_field`, `number_precision`, `first_month_of_fiscal_year`
- Title-field children: `selection_mode`, `field_code`
- Number-precision children: `total_digits`, `decimal_places`, `rounding_mode`
- Feature toggles: `thumbnails_enabled`, `bulk_deletion_enabled`, `comments_enabled`,
  `record_duplication_enabled`, `inline_record_editing_enabled`

Provider authentication attributes and data-source output attributes are deliberately not reserved here.
Issue #11 must choose them under this convention when their schemas are designed.
