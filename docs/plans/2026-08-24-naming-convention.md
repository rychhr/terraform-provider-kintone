# Provider Naming Convention Implementation Plan

> **For maintainers:** Implement this plan on `docs/define-naming-convention` before the v0.1.0 provider
> schema is implemented.

**Goal:** Define the durable naming convention for public provider interfaces and reserve the v0.1.0
resource, data source, and app-schema names.

**Architecture:** Keep enforceable naming rules and examples in `CONTRIBUTING.md`, and keep the rationale,
alternatives, and scope decisions in a design specification. Update the README to point contributors at
the convention. This is a documentation-only change; provider code, schemas, and generated documentation
remain unchanged.

**Reference:** [HashiCorp provider naming conventions](https://developer.hashicorp.com/terraform/plugin/best-practices/naming)

**Spec:** `docs/specs/2026-08-24-naming-convention-design.md`

---

## Task 1: Document and validate the provider naming convention

**Files:**

- Create: `docs/specs/2026-08-24-naming-convention-design.md`
- Create: `CONTRIBUTING.md`
- Modify: `README.md`

1. Document why public provider names must be decided before publication and separate the durable
   contributor rules from the design rationale.
2. Base the convention on HashiCorp's provider naming guidance: use English lowercase `snake_case`, write
   acronyms such as `api`, `acl`, `id`, and `url` in lowercase, name managed payloads with nouns, use
   singular names for single objects and plural names for collection-wide singleton resources, use
   singular data-source names for one object and plural names for lists, and qualify concepts by placing
   the direct target first, as in `app_acl`, `record_acl`, and `field_acl`.
3. Require singular names for single-value attributes, plural names for collections, `<object>_id` for
   reference identifiers, and affirmative `*_enabled` names for boolean state.
4. Define `kintone_app` as the owner of app identity, create-only placement, and general settings. Define
   any other independently managed kintone settings surface that exists at most once per app as a
   standalone singleton resource whose import ID is the app ID. State that a separate API endpoint alone
   is not a reason to split a resource.
5. Show that the rules classify a single entity, a collection-wide singleton, app/record/field ACLs, and
   general settings without case-by-case exceptions. Treat later-roadmap resource names only as examples,
   not reserved interfaces.
6. Reserve these v0.1.0 public names for Issue #11:
   - managed resource: `kintone_app`;
   - single data source: `kintone_app`;
   - list data source: `kintone_apps`;
   - identity and lifecycle attributes: `id`, `name`, `description`, `space_id`, `thread_id`, `revision`;
   - general settings: `theme`, `title_field`, `number_precision`, `first_month_of_fiscal_year`;
   - title-field children: `selection_mode`, `field_code`;
   - number-precision children: `total_digits`, `decimal_places`, `rounding_mode`; and
   - feature toggles: `thumbnails_enabled`, `bulk_deletion_enabled`, `comments_enabled`,
     `record_duplication_enabled`, `inline_record_editing_enabled`.
7. Leave provider authentication attributes and data-source output attributes for Issue #11 to decide
   under the same convention.
8. Record rejected alternatives in the design: mirroring kintone API casing or endpoint boundaries,
   making every singleton name singular regardless of payload cardinality, embedding every settings
   surface in `kintone_app`, and reserving later-roadmap names before their designs exist.
9. Replace the README's promise of a future naming document with a link to `CONTRIBUTING.md`.
10. Run `git diff --check`, `make build`, `make test`, `make lint`, and `make docs`; confirm generated
    documentation has no remaining diff.
11. Run the documented redacted gitleaks scans for the working tree, branch diff, and intended commit
    message. Commit the change as `docs: define provider naming convention`.
12. Before requesting push approval, inspect the staged diff, working tree, ignored `.env.local` state,
    and an English pull-request body that includes `Closes #5`, the verification commands, and the fact
    that schema, state, credentials, generated documentation, acceptance tests, API calls, and remote
    infrastructure are unaffected.
