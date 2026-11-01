Unofficial CLI for the PLANKA REST API. Not affiliated with or endorsed by PLANKA Software GmbH.

Use of a PLANKA server is governed by PLANKA's own licence; commercial/hosted use may require a PLANKA Pro/Enterprise licence.

# kanbancli

`kanbancli` is a small Go CLI for the PLANKA REST API. It is **unofficial** and **not affiliated with or endorsed by PLANKA Software GmbH**. It targets humans and AI agents that want structured JSON (or tables) instead of wiring an MCP server — see [plankanban/planka#1247](https://github.com/plankanban/planka/issues/1247).

MIT licensed. Third-party notices in [NOTICE](NOTICE); release archives also ship `THIRD_PARTY_NOTICES` and `third_party_licenses/`.

## Install

Or install from source with Go **1.26.10** (matches `go.mod`):

```bash
go install github.com/swtormy-agents/kanbancli/cmd/kanbancli@latest
```

Release archives (Linux and Windows, amd64/arm64) are published on [GitHub Releases](https://github.com/swtormy-agents/kanbancli/releases) when tagged.

## Quick start

```bash
export KANBANCLI_URL=https://kanban.example.com
export KANBANCLI_TOKEN=...    # access token or API key — never pass on the command line

kanbancli health
kanbancli projects list
kanbancli boards get --id 1
kanbancli cards list --board 1
kanbancli users list
```

Obtain a token interactively (password from env only):

```bash
export KANBANCLI_EMAIL=you@example.com
export KANBANCLI_PASSWORD=...
kanbancli auth login --show-token   # prints token only to stdout
```

## Output

- Default: JSON on stdout; one JSON error object on stderr
- `--output table` for simple tabular views
- `--jq '...'` to filter JSON (gojq)
- `--pretty` to indent JSON

## Safety

- Never log or print the API token or password; `--dry-run` and errors redact secrets as `<API_TOKEN>` / `<REDACTED>`
- Agents stay read-only. Mutating commands and non-GET `api` calls need `--write`, a TTY, and typing `write` when prompted
- Exit codes: `0` ok, `1` runtime/API, `2` usage or config, `3` auth, `4` blocked

See [docs/api-scope.md](docs/api-scope.md) for endpoints covered in v0.1.

## Agent skills

Bundled skill (also in [skills/kanbancli/SKILL.md](skills/kanbancli/SKILL.md)):

```bash
kanbancli skill install --agent cursor
kanbancli skill print
```

## License

MIT — see [LICENSE](LICENSE).
