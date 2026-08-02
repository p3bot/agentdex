# agentdex

agentdex is a Go library plus thin CLI that indexes three kinds of data — AI coding agents, the models.dev providers that power them, and the models those providers offer — and serves all three as browsable data. For an agent it reports the binary, version, config and skills directories, providers, and available models (enriched from models.dev), and whether it is installed on the local machine; providers and models are browsable in their own right. It owns the outside of an agent — identity, location, paths, version, capability — and never reads an agent's internal configuration.

## Module layout

The repository hosts two independent module systems that do not interfere:

- Go module at the repository root: `github.com/start-cli/agentdex`. The index library (root package `agentdex`), the public models.dev subpackage (`modelsdev/`), private subsystems under `internal/`, and the CLI under `cmd/agentdex/`.
- CUE module under `catalog/`: `github.com/start-cli/agentdex/catalog@v1`. The `#KnownAgent` schema and the agent catalog data, published to the CUE Central Registry and fetched at runtime. It is versioned and published independently of the Go binary.

The Go build ignores `catalog/`; the CUE module is a self-contained CUE module with its own `cue.mod/module.cue`.

## Project documents

Feature work lives in the ambient `pj` scope `ad` under `.agents/projects/`. Board order is chronological (`pj list --all` reads as history). Read verbs: `pj list`, `pj status`, `pj get`, `pj search`, `pj next`. Resolve a former root filename via the `legacy` frontmatter field:

```bash
rg -l 'legacy: "01-theme-safe-terminal-path-colour.md"' .agents/projects
```

## Toolchain and platforms

- Go 1.25. Pure Go: no cgo, no C dependencies. The binary must build with `CGO_ENABLED=0`.
- CUE module language version pinned to `v0.16.0`. Do not use CUE features beyond that pin.
- Target platforms: Linux, macOS, and WSL (agents installed natively in the WSL Linux environment). No native Windows, and no Windows-host agents reached through WSL PATH interop.
- XDG base directories are resolved from the published environment variables with the documented home fallbacks, not from platform-specific user-dir helpers.
- Dependencies are resolved to latest at build time. Each new dependency is a standing liability; prefer the standard library and dependencies already carried.

## Catalog delivery and caching

The agent catalog is not embedded in the binary. It is fetched from the CUE Central Registry at runtime using `cuelang.org/go/mod/modconfig`, which honours `CUE_REGISTRY` and `cue login` with no agentdex-specific auth settings. The schema travels with the data: the published module bundles its own `schema.cue`, so the loader validates by evaluating the fetched module rather than carrying a schema of its own.

Caching is version-resolution caching layered over CUE's own module content cache, not a JSON snapshot of the catalog:

- agentdex caches the resolved catalog version under `$XDG_CACHE_HOME/agentdex/`, keyed by module path, for a TTL (24h default), and relies on CUE's content cache for the module data.
- Resolving the latest version (`ModuleVersions`) requires the network. Fetching a canonical `module@version` is served from CUE's content cache offline. This is the basis for the two behaviours below.
- On a failed re-resolution after the TTL expires, agentdex keeps using the last resolved version (the resolution is reported as stale so a caller can warn while still working).
- A first run with no network and no previously resolved version fails clearly with `ErrCatalogUnavailable`. This is accepted behaviour, not a defect.

## Adding an agent to the catalog

An agent is a catalog edit, not a code change. Each entry is one map key in
`catalog/agents.cue`; the key is the kebab-case id (`^[a-z0-9]+(-[a-z0-9]+)*$`)
and the single source of identity, so there is no `id` field inside the entry.
Report only the outside of the agent (identity, location, paths, version,
capability); never add a field that requires reading the agent's internal
configuration.

### 1. Research the outside facts

Gather the static facts the catalog stores and confirm each against the real
agent, not from memory. Prefer primary sources in this order:

- Run the installed binary: resolve it on PATH, print version and help, and use
  any inspect or doctor command that reports discovered paths.
- Read the product's own docs (README, user guide, file-locations pages) for
  config and skills roots. Prefer docs shipped with the install when present.
- If the agent is open source, clone the repository and inspect the source for
  path constants, discovery order, version flags, and provider defaults. Treat
  source as authoritative when docs and binary disagree.
- Confirm each `provider` id against models.dev (provider page or API catalog).
  A wrong id silently drops model enrichment.

Facts to collect:

- `bin`: the executable name resolved on PATH (`exec.LookPath`), no `.exe`.
- `config` paths: global and optional local directories, written with `~` and
  XDG-style paths, not an absolute home.
- `skills` paths: omit entirely when the agent has no skills directories. When
  present, classified roots per scope (`global` = user-wide, `local` =
  project). Within each scope set the roles the agent supports: `agents`
  (shared `~/.agents/skills` or `.agents/skills`), `native` (this product's own
  tree, e.g. `~/.claude/skills`), and `alternatives` (other supported roots).
  Written with `~` and XDG-style paths. See `docs/agents-skills-path-matrix.md`.
  Do not store `primary` — the library derives it as agents, else native, else
  the first alternative. Order `alternatives` by preference: when agents and
  native are unset, `alternatives[0]` becomes primary. Empty `skills: {}` or an
  empty scope is rejected by the schema (`struct.MinFields(1)`).
- `version.args` and optional `version.pattern`: the flag that prints the
  version and a regex to extract it. Run the binary once and check that the
  pattern captures the version string from real output before cataloguing it.
- `provider`: one or more real models.dev provider ids. This is the join key to
  models.dev enrichment; a wrong id silently drops model data. An agent that can
  drive any models.dev provider (e.g. opencode) is provider-agnostic: set
  `agnostic: true` and omit `provider` entirely — the schema rejects an entry
  that has both. Callers supply the enrichment set at query time via
  `--provider`; never infer it from the agent's internal configuration.

Config is a single global/local pair. Skills use agents / native / alternatives
per scope so the main product answer (primary) stays a stable path while the
full support set remains expressible. Keep the native location for `config`
when the agent has no `.agents` mapping for that slot.

### 2. Add the entry

Add a block alongside the existing agents in `catalog/agents.cue`:

```cue
agents: "claude-code": {
	name:        "Claude Code"
	bin:         "claude"
	description: "Anthropic's agentic coding tool that runs in the terminal."
	config: {
		global: "~/.claude"
		local:  ".claude"
	}
	skills: {
		global: {native: "~/.claude/skills"}
		local:  {native: ".claude/skills"}
	}
	version: {
		args:    ["--version"]
		pattern: "([0-9]+\\.[0-9]+\\.[0-9]+)"
	}
	provider: ["anthropic"]
	homepage: "https://github.com/anthropics/claude-code"
}
```

An agnostic entry (e.g. opencode) sets `agnostic: true` and omits `provider`.

When the entry has `skills`, update `docs/agents-skills-path-matrix.md` with a
section for the agent that lists each supported skills root and whether it is
supported (Yes / No / n/a). Keep the matrix aligned with the catalog roles:
agents, native, and alternatives for both global and local scopes.

Fields, per `catalog/schema.cue`:

| Field | Required | Notes |
|---|---|---|
| `name` | yes | Human display name, non-empty |
| `bin` | yes | Executable resolved on PATH, non-empty |
| `config.global` | yes | Global config directory |
| `provider` | unless agnostic | One or more models.dev provider ids; the join key; forbidden when `agnostic` is true |
| `agnostic` | no | Defaults false; true marks a provider-agnostic agent with no home provider |
| `description` | no | One sentence |
| `config.local` | no | Project-local config directory |
| `skills.global` / `skills.local` | with `skills` | At least one scope; each is a `#SkillsScope` |
| `skills.*.agents` | no | Shared `.agents` skills path when supported |
| `skills.*.native` | no | This product's own skills path when supported |
| `skills.*.alternatives` | no | Other supported roots (compat trees, etc.); priority order, first is primary fallback |
| `version.args` | no | Appended to the binary, e.g. `["--version"]` |
| `version.pattern` | no | Regex to extract the version string |
| `homepage` | no | Project URL |

### 3. Validate locally

From `catalog/`:

```bash
cue vet ./...
cue mod tidy
```

`cue vet` validates by evaluation because `schema.cue` travels with the data; a
missing required field or an empty path string fails schema constraints (e.g.
`!=""`), and empty `skills: {}` or an empty scope (`skills: { global: {} }`)
fails via `struct.MinFields(1)` on skills and each scope. `cue mod tidy` must
leave the module clean.

### 4. Exercise through the library

Point the loader at the working-tree catalog with the `catalog.dir` key in
config, rather than the registry. Config lives at
`$XDG_CONFIG_HOME/agentdex/config.cue` (fallback `~/.config/agentdex/config.cue`).
`catalog.dir` loads the catalog by evaluating the local CUE module directory: no
version is resolved and no registry is contacted, so the unpublished working
tree is visible immediately, and an entry the schema rejects fails here with the
CUE diagnostic naming it. A module path cannot name an unpublished tree, so this
is the only source that shows a working edit before it is published. Set the key
to the repository's `catalog/` directory:

```cue
catalog: dir: "/path/to/agentdex/catalog"
```

For an isolated check that does not touch the user config, point
`XDG_CONFIG_HOME` at a temporary directory that contains
`agentdex/config.cue` with that `catalog.dir` value.

Then confirm the new entry before publishing. `agents list` shows the whole
catalog, so the added agent appears with its detection status even before it is
installed; `--installed` narrows to the agents present on this machine:

```bash
agentdex agents list
agentdex agents get <id>
```

Check that version extraction, config and skills paths, and provider model
counts look right on `agents get` before publishing.

### 5. Publish a new catalog version

The catalog is versioned and published independently of the Go binary, so adding
an agent needs no agentdex release. Publish a new version under the `@v1` major
line to the CUE Central Registry with the same mechanism start/library uses;
`cue login` and `CUE_REGISTRY` are honoured as-is, with no agentdex-specific auth.
Existing installs resolve the new version within the cache TTL (24h default); new
installs resolve it immediately via `ModuleVersions`.

Schema-breaking catalog changes are not independent of the binary. The schema
travels with the data, so a new `#KnownAgent` shape that older loaders cannot
decode will fail every install that re-resolves after the TTL. Publish those
catalog versions only together with a matching agentdex release, and do not
publish the catalog alone ahead of the binary. Additive entry changes (new
agents, path or provider edits within the existing schema) stay catalog-only.

## Style

Go:

- Match the surrounding code's conventions. Push nondeterministic inputs (clock, filesystem, network, environment) to the boundary so core logic is testable from inputs.
- Comments document why, not what. Respect godoc form on exported symbols.
- Tests favour real behaviour over mocks: real CUE validation, real files via `t.TempDir()`, environment isolation via `t.Setenv`. Table-driven tests for multiple cases.

Markdown for agent-facing documents (`AGENTS.md`, pj project bodies, design notes):

- No bold or italic, no horizontal rules, no emojis, no HTML comments.
- No heading depth beyond `###`. No directory structures beyond depth 3.
- Single blank lines between sections. Inline code, code blocks, tables, and lists are fine.

## Finalisation sweep

Before declaring work complete, run from the repository root:

- `gofmt -l .` (and `go fix ./...` where applicable) — formatting clean.
- `go build ./...` and `go vet ./...` — pass.
- `golangci-lint run` — clean.
- `go test ./...` — pass.
- For the catalog CUE module: `cue vet ./...` from `catalog/`, and `cue mod tidy` leaves it clean.

## Commit convention

Scoped Commits (https://scopedcommits.com).

- Format: `<scope>: <description>`, optional body, optional trailers.
- Scope is the subsystem, module, or area touched (e.g. `catalog`, `loader`, `docs`).
- Multiple scopes comma-separated. No `feat`/`fix` type prefix.
