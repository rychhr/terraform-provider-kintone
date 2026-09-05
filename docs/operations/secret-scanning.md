# Secret scanning

Read this before installing hooks, scanning changes, changing scanner rules, or handling a finding.
Run commands from the repository root. The approval and public-information rules in
[AGENTS.md](../../AGENTS.md) still apply.

`gitleaks` runs over every commit through `pre-commit`, and again in CI. Install the hooks once per clone:

```sh
pre-commit install --hook-type pre-commit --hook-type commit-msg
```

Both types are required, and `commit-msg` is not a type `pre-commit install` reaches for on its own —
`.pre-commit-config.yaml` sets `default_install_hook_types` so that the bare command installs both anyway,
but the explicit form above does not depend on that file being read as expected. The `commit-msg` hook is
the one this setup exists for: `gitleaks git` scans diffs, not commit messages, so a secret or an agent
session link placed only in a message is invisible to the staged-content hook. The `commit-msg` hook hands
the message file to `gitleaks dir`, which does see it.

The ruleset is `.gitleaks.toml` — the gitleaks defaults plus `agent-session-link`, which matches
`claude.ai/code/session_` followed by an id. It deliberately does not match the placeholder forms quoted
in the agent-session-link rule in [AGENTS.md](../../AGENTS.md#workflow).

`.github/workflows/secret-scan.yml` runs the same configuration over the commits an event introduces —
their diffs as well as their messages, because a secret added in one commit and removed in the next leaves
the tip tree clean while the blob stays retrievable by its SHA. It scans that range rather than the whole
history: a session link reached a commit message here before the rule existed, and rewriting that history
would buy nothing, for the same reason.

Three commands run the scan by hand:

| Command | Scans |
| --- | --- |
| `scripts/scan-commit-messages.sh origin/main..HEAD` | the commit messages a range selects |
| `gitleaks git . --config .gitleaks.toml --redact --log-opts=origin/main..HEAD` | the diffs in that range |
| `gitleaks dir . --config .gitleaks.toml --redact` | file content in the working tree |

Keep `--redact` on a manual run. `gitleaks dir` does not honour `.gitignore`, so it reads `.env.local`
along with everything else, and without redaction a real development password lands in terminal scrollback.

`pre-commit run --all-files` is not a whole-tree scan: the staged-content hook passes no file names and
reads the staged diff, so with a clean working tree it reports success having read nothing. The
`gitleaks dir` command above is the whole-tree scan.

`scripts/test-gitleaks-rules.sh` is the ruleset's own regression test. Run it after editing
`.gitleaks.toml`. Its fixtures are assembled at run time in a temporary directory, because a file in this
repository holding a real-form session link or access key would be flagged by the scanner under test.

## Bypassing a hook

Bypassing is a decision to record, not a habit to fall into. Establish that the hit is a false positive
before reaching for either route below; a real finding is fixed by removing the string, never by silencing
the scanner.

- `git commit --no-verify` skips both hooks for one commit. It does not skip CI, which runs the same
  configuration over the branch's diffs and messages and will fail the pull request.
- A false positive that has to stay in the tree needs the scanner to accept it everywhere: add a
  `gitleaks:allow` comment on the line, or record the finding's fingerprint in a `.gitleaksignore` file.
  Say in the pull request what was allowed and why.
