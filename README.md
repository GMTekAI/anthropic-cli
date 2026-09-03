# ant — Claude Platform CLI

[![GitHub release](https://img.shields.io/github/v/release/anthropics/anthropic-cli)](https://github.com/anthropics/anthropic-cli/releases)
[![Homebrew](https://img.shields.io/badge/homebrew-anthropics%2Ftap%2Fant-FBB040?logo=homebrew&logoColor=white)](https://github.com/anthropics/homebrew-tap)

`ant` is the official CLI for the [Claude Platform](https://platform.claude.com/docs/en/api). It puts the Claude API in your terminal — send messages, manage agents and sessions, upload files, and script against every API endpoint.

![Demo of the ant CLI](.github/demo.gif)

## Documentation

Full documentation is available at **[platform.claude.com/docs/en/api/sdks/cli](https://platform.claude.com/docs/en/api/sdks/cli)**.

<!-- x-release-please-start-version -->

## Installation

### Homebrew

```sh
brew install anthropics/tap/ant
```

### Go

To install from source, you need [Go](https://go.dev/doc/install) version 1.22 or later.

```sh
go install 'github.com/anthropics/anthropic-cli/cmd/ant@latest'
```

The binary is placed in `$(go env GOPATH)/bin`. If `ant` isn't found after installation, add that directory to your `PATH`:

```sh
# Add to your shell profile (.zshrc, .bashrc, etc.)
export PATH="$PATH:$(go env GOPATH)/bin"
```

<!-- x-release-please-end -->

## Getting started

Log in with your Claude Console account:

```sh
ant auth login
```

Or set the `ANTHROPIC_API_KEY` environment variable to an API key from the [Claude Console](https://platform.claude.com/settings/keys).

To hand the CLI a key from a secret manager without putting it in the environment or on the command line, pipe it on stdin:

```sh
op read op://vault/anthropic/api-key | ant --api-key-stdin models list
```

Passing a credential as `--api-key <value>` / `--auth-token <value>` is deprecated: the value is visible in shell history and process listings.

Then send your first message:

```sh
ant messages create \
  --model claude-opus-4-8 \
  --max-tokens 1024 \
  --message '{role: user, content: "Hello, Claude"}'
```

Structured flags accept relaxed JSON or YAML, so unquoted keys are fine.

## Usage

The CLI follows a resource-based command structure, with nested resources separated by colons:

```sh
ant <resource>[:<subresource>] <command> [flags...]
```

```sh
# List available models
ant models list

# Browse a response in the interactive explorer (the default in a terminal)
ant models retrieve --model-id claude-opus-4-8

# Extract a single field from a response, jq-style
ant messages create \
  --model claude-opus-4-8 \
  --max-tokens 1024 \
  --message '{role: user, content: "Hello, Claude"}' \
  --transform content.0.text --raw-output

# Send a file using the @path syntax
ant messages create \
  --model claude-opus-4-8 \
  --max-tokens 1024 \
  --message '{role: user, content: [
    {type: image, source: {type: base64, media_type: image/jpeg, data: "@photo.jpg"}},
    {type: text, text: "What is in this image?"}
  ]}'

# Manage beta resources such as agents, sessions, and files
ant beta:agents list
```

Run `ant --help` for the full list of resources, or append `--help` to any command to see its flags.

## Managing agents as code

`ant apply` keeps agents, skills, environments, memory stores and deployments
in step with files in your repository, so changes to them go through review
like any other code. Files reference each other by path rather than by ID.

Apply records which remote object each file became in `claude-lock.json`.
Commit it with the files, so teammates and CI update the same resources
instead of creating their own copies:

```json
{
  "version": 1,
  "origin": {
    "base_url": "https://api.anthropic.com",
    "organization_id": "1a5099b3-3dc8-4f5e-9d27-58cd1e7b40a1",
    "workspace_id": "wrkspc_01JwQvzn5eR6bTzHkAqzYtGZ"
  },
  "resources": {
    "./agents/code-reviewer.md": {
      "kind": "agent",
      "id": "agent_011CZkYqphY8vELVzwCUpqiQ",
      "version": "3",
      "hash": "d23251c8d99b3613a64f3f8d87f5fad4",
      "remote_hash": "1b771bee5bdbf600a5ad972fdac32d94"
    },
    "./environments/cloud.yml": {
      "kind": "environment",
      "id": "env_011CZkZ9X2dpNyB7HsEFoRfW",
      "hash": "9369a9f39b013ae19aa9d23221c0fbc2",
      "remote_hash": "02a44853c8cbc31b095295c5a286ff27"
    }
  }
}
```

`hash` fingerprints what was last sent, so a changed file is noticed;
`remote_hash` fingerprints what the server held afterwards, so an edit made in
the Console is noticed too.

```
your-repo/
├── claude-lock.json
├── agents/
│   ├── code-reviewer.md
│   └── code-verifier.md
├── deployments/
│   └── nightly-review.md
├── environments/
│   └── cloud.yml
└── skills/
    └── pr-writer/
        └── SKILL.md
```

The directory a file is in decides what kind of resource it is, and a skill is
any directory containing a `SKILL.md`. A `.yml` file contains the API request
body as-is. A `.md` file puts the request body in its frontmatter and uses the
markdown text as the agent's system prompt (for a deployment, its first user
message; for an environment, its description). Wherever the API expects another
resource's ID, write the path to that resource's file instead:

```markdown
---
model: claude-sonnet-4-5
skills:
  - ../skills/pr-writer          # also accepts a glob, or a GitHub .../tree/<branch>/<dir> URL
  - {type: anthropic, skill_id: xlsx}   # anything that is not a path is sent to the API unchanged
tools:
  - type: agent_toolset_20260401
  - ./tools/*.json               # the contents of these files are inserted into the list here
multiagent:
  type: coordinator
  agents:
    - ./code-verifier.md
---
Review the pull request. Delegate verification to the code-verifier agent.
```

```markdown
---
# deployments/nightly-review.md
agent: ../agents/code-reviewer.md
environment_id: ../environments/cloud.yml
schedule: {type: cron, expression: "0 3 * * *", timezone: America/Los_Angeles}
---
Review any open pull requests. Start with the oldest.
```

Apply prints the plan below and asks before changing anything. Press `d` at the
prompt to see each change field by field, or run with `--dry-run` to print the
plan without applying it.

```console
$ ant apply ./agents ./deployments ./environments ./skills
Preview  ./claude-lock.json

± Name                       Plan
+ ./skills/pr-writer         create
~ ./agents/code-verifier.md  update     [~system]
~ ./agents/code-reviewer.md  update     [~multiagent]

Resources  + 1 to create · ~ 2 to update · 2 unchanged

Apply these changes? (y)es / (n)o / (d)etails
```

- **References record a specific version.** A coordinator stores its
  sub-agent's ID and version, so when `code-verifier.md` changes above,
  `code-reviewer.md` is updated in the same run to point at the new version. A
  skill referenced by GitHub URL keeps using the commit it resolved to on the
  first apply, even after the branch moves on; run with `--upgrade` to pick up
  the branch's latest commit.
- **Edits made in the Console are not silently overwritten.** If a resource was
  changed, archived or deleted there since the last apply, the plan points it
  out and refuses to continue. Re-run with `--force` to overwrite it with what
  the file says.
- **Deleting a file leaves the resource in place.** Run with `--prune` to also
  remove resources whose files are gone.

Running `ant apply` with no paths reconciles every resource already in the
lockfile. `ant apply --help` lists all flags.

### In CI

This workflow runs apply on every push to `main`. A protected GitHub
environment controls who can deploy, workload identity federation lets the job
authenticate without storing an API key, and the last step commits the updated
lockfile so the next run knows what this one created.

```yaml
name: agents
on:
  push:
    branches: [main]
    paths-ignore: [claude-lock.json]   # so the lockfile commit at the end does not trigger this workflow again
jobs:
  apply:
    runs-on: ubuntu-latest
    environment: agents-prod           # protected: required reviewers, main only
    concurrency: ant-apply             # one run at a time: apply does not guard against concurrent runs itself
    permissions: {id-token: write, contents: write}
    steps:
      - uses: actions/checkout@v4
      - uses: anthropics/setup-ant@v1
      - name: Mint OIDC token
        run: |
          curl -sH "Authorization: bearer $ACTIONS_ID_TOKEN_REQUEST_TOKEN" \
            "$ACTIONS_ID_TOKEN_REQUEST_URL&audience=anthropic" | jq -r .value > "$RUNNER_TEMP/oidc"
      - name: Apply
        env:
          ANTHROPIC_IDENTITY_TOKEN_FILE: ${{ runner.temp }}/oidc
          ANTHROPIC_FEDERATION_RULE_ID: ${{ vars.ANT_FEDERATION_RULE_ID }}
          ANTHROPIC_ORGANIZATION_ID: ${{ vars.ANT_ORG_ID }}
        run: ant apply --yes
      - name: Commit the lockfile
        run: |
          git diff --quiet claude-lock.json && exit 0
          git -c user.name="github-actions[bot]" -c user.email="github-actions[bot]@users.noreply.github.com" \
            commit -am "Update claude-lock.json [skip ci]"
          git push
```

## Requirements

macOS, Linux, or Windows.

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md).

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
