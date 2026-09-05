# Terraform Provider for kintone

A Terraform provider for managing [kintone](https://www.kintone.com/) apps and their settings
declaratively. It is intended to be published to the Terraform Registry as `rychhr/kintone`, and is built on
[terraform-plugin-framework](https://github.com/hashicorp/terraform-plugin-framework) using plugin protocol
v6.

> **Status: implemented, awaiting Terraform Registry publication.**
> The provider core, app resource, and data sources are implemented. Nothing has been published to the
> Registry yet, so use a development override to try the provider locally. The generated documentation
> below describes the current schema.

## Requirements

- Terraform CLI **1.16.0 or later**. CI tests 1.16.0 and the latest stable release.
- A kintone subdomain, and an account that may administer apps — see the prerequisites below.

## kintone prerequisites

The kintone REST API and the provider's supported authentication methods affect setup and operation.

**App creation through this provider requires password authentication.** The provider supports password
and API-token authentication. The create API excludes API tokens and also supports session and OAuth
authentication, which this provider does not implement. Use password authentication over the
`X-Cybozu-Authorization` header. Accounts with two-factor authentication enabled cannot authenticate this
way. Use a dedicated service account without 2FA, granted only the app-administration permissions it needs.

**There is no API for deleting an app.** Removing a managed app from your configuration removes it from
Terraform state only — the app itself remains in kintone and has to be deleted by hand. The provider will
report what needs manual cleanup rather than silently leaving it behind. Take this into account before
running the provider against a shared subdomain.

App settings are also written in two phases: changes go to kintone's *preview* environment and are then
deployed, so a single `terraform apply` performs a write followed by a deployment that the provider waits
on.

## Usage (v0.1.0)

The v0.1.0 release is deliberately a minimal core, so the Registry publishing path is exercised end to end
before feature work begins:

- [Provider configuration](docs/index.md) — password authentication and API-token authentication,
  configurable in HCL or through `KINTONE_BASE_URL`, `KINTONE_USERNAME`, `KINTONE_PASSWORD`, and
  `KINTONE_API_TOKENS`
- [`kintone_app`](docs/resources/app.md) — an app and its general settings
- [`data.kintone_app`](docs/data-sources/app.md) — a published app and its settings
- [`data.kintone_apps`](docs/data-sources/apps.md) — published apps matching filters

Once published, the provider will be required like this:

```hcl
terraform {
  required_version = ">= 1.16.0"

  required_providers {
    kintone = {
      source = "rychhr/kintone"
    }
  }
}
```

The linked generated documentation lists the current schema and examples. Public names follow the
[naming convention](CONTRIBUTING.md#naming-public-provider-interfaces) and cannot change after publication.

## Roadmap

| Version | Scope |
| --- | --- |
| v0.1.0 | provider authentication, `kintone_app`, data sources, Registry publishing path |
| v0.2.0 | form fields and form layout |
| v0.3.0 | views and process management |
| v0.4.0 | the three ACL resources |
| v0.5.0 | notification resources, customization, actions, reports, admin notes, and app icon |
| v1.0.0 | once the schema is stable |

Each minor release gets its own design before implementation starts.

## Development

Build with `go build -o terraform-provider-kintone .` and point Terraform at the resulting binary through
a development override in your Terraform CLI configuration file. See the
[development build procedure](docs/operations/release-artifacts.md) for artifact requirements:

```hcl
provider_installation {
  dev_overrides {
    "rychhr/kintone" = "/path/to/directory/containing/the/binary"
  }
  direct {}
}
```

Terraform derives the executable name from the **last element of the source address**: it looks for
`terraform-provider-<TYPE>`, where `<TYPE>` is that last element. For the address `rychhr/kintone` the
binary must therefore be named `terraform-provider-kintone`. With a development override in place,
`terraform init` is skipped for this provider — run `terraform plan` directly.

### Secret scanning hooks

The repository scans commits with [gitleaks](https://github.com/gitleaks/gitleaks) through
[pre-commit](https://pre-commit.com/), so that a credential — or an agent session link — cannot reach a
commit. Install the hooks once per clone:

```sh
pre-commit install --hook-type pre-commit --hook-type commit-msg
```

Both hook types are needed, and `commit-msg` is not one `pre-commit install` reaches for on its own. The
configuration asks for both, so the bare command installs both, but the explicit form above says so rather
than relying on it. The same configuration runs in CI on every pull request — over the branch's diffs and
commit messages, not only the final tree — so a commit that skips the hooks is caught there instead. The
ruleset and the bypass path are described under [Secret scanning](AGENTS.md#secret-scanning) in
`AGENTS.md`.

Repository conventions, build and test commands, and the API constraints that implementations must respect
are documented in [AGENTS.md](AGENTS.md). The kintone API behavior behind the prerequisites above — the
preview-and-deploy path, the missing deletion API, and the authentication rules — is written up with its
sources in [docs/design/kintone-api-constraints.md](docs/design/kintone-api-constraints.md).

## Contributing

Issues and pull requests are welcome and are written in English. Please read
[AGENTS.md](AGENTS.md) and [CONTRIBUTING.md](CONTRIBUTING.md) before contributing; the latter defines the
naming convention for public resources, data sources, and attributes.

The [acceptance-test procedure](CONTRIBUTING.md#acceptance-tests) describes dedicated development
credentials, the existing token-test app, explicit approval, and manual cleanup. Never run acceptance
tests against a production subdomain.

## License

Licensed under the Mozilla Public License 2.0. See [LICENSE](LICENSE) for the full text.
