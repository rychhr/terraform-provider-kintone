# Terraform Provider for kintone

A Terraform provider for managing [kintone](https://www.kintone.com/) apps and their settings
declaratively. It is intended to be published to the Terraform Registry as `rychhr/kintone`, and is built on
[terraform-plugin-framework](https://github.com/hashicorp/terraform-plugin-framework) using plugin protocol
v6.

> **Status: pre-implementation.**
> This repository is a clean rewrite and does not contain provider code yet. Nothing has been published to
> the Terraform Registry, so the provider cannot be installed with `terraform init` at this point. Anything
> below marked *planned* describes the intended v0.1.0 surface, not shipped behavior.

## Requirements

- A Terraform CLI that supports plugin protocol v6.
- A kintone subdomain, and an account that may administer apps — see the prerequisites below.

## kintone prerequisites

Two properties of the kintone REST API affect how you set up and operate this provider. Neither is a choice
made by the provider.

**App creation requires password authentication.** The kintone API accepts an API token for many
operations, but creating an app is not one of them; it requires password authentication over the
`X-Cybozu-Authorization` header. Accounts with two-factor authentication enabled cannot authenticate this
way. Use a dedicated service account without 2FA, granted only the app-administration permissions it needs.

**There is no API for deleting an app.** Removing a managed app from your configuration removes it from
Terraform state only — the app itself remains in kintone and has to be deleted by hand. The provider will
report what needs manual cleanup rather than silently leaving it behind. Take this into account before
running the provider against a shared subdomain.

App settings are also written in two phases: changes go to kintone's *preview* environment and are then
deployed, so a single `terraform apply` performs a write followed by a deployment that the provider waits
on.

## Planned usage (v0.1.0)

The v0.1.0 release is deliberately a minimal core, so the Registry publishing path is exercised end to end
before feature work begins:

- provider configuration — password authentication and API-token authentication, configurable in HCL or
  through the `KINTONE_BASE_URL`, `KINTONE_USERNAME`, and `KINTONE_PASSWORD` environment variables
- `kintone_app` — an app and its general settings
- `data.kintone_app` and `data.kintone_apps`

Once published, the provider will be required like this:

```hcl
terraform {
  required_providers {
    kintone = {
      source = "rychhr/kintone"
    }
  }
}
```

Resource and attribute schemas are not finalized and are therefore not documented here. Names cannot be
changed once they are published to the Registry, so they are being settled against a written naming
convention rather than case by case. Generated provider documentation will accompany the first release.

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

Once the provider code lands, build it with `go build` and point Terraform at the resulting binary through
a development override in your Terraform CLI configuration file:

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
[AGENTS.md](AGENTS.md) first; a `CONTRIBUTING.md` covering the naming convention for resources and
attributes will be added alongside the first release.

Acceptance tests create real apps in a kintone environment and, because there is no deletion API, leave
them behind for manual cleanup. Never run them against a production subdomain.

## License

Licensed under the Mozilla Public License 2.0. See [LICENSE](LICENSE) for the full text.
