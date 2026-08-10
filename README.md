# agentdex

agentdex indexes AI coding agents, the [models.dev](https://models.dev) providers that power them, and the models those providers offer — and serves all three as browsable data.

For each catalogued agent it reports the outside facts: whether it is installed, where its binary lives, its version, config and skills directories, which model provider(s) it uses, and (from models.dev) the models available to it. Providers and models are queryable on their own. It never reads an agent's internal configuration, so other tools can resolve paths and capability without hardcoding product layouts.

Ships as a Go library and a thin CLI over it. Only catalogued agents are reported (never arbitrary PATH executables). The catalog is fetched from the CUE Central Registry at runtime and cached, so the known-agent set can change without an agentdex release.

## Install

Targets Linux, macOS, and WSL only (see [Platforms](#platforms)). First run needs network access to resolve the agent catalog (and models.dev when enrichment is requested). Later runs can work offline from cache; see [Catalog and caching](#catalog-and-caching).

### CLI

Homebrew (Linux/macOS):

```bash
brew tap p3bot/tap
brew trust p3bot/tap
brew install p3bot/tap/agentdex
```

Go:

```bash
go install github.com/p3bot/agentdex/cmd/agentdex@v1.0.0
```

### Library

```bash
go get github.com/p3bot/agentdex@v1.0.0
```

```go
require github.com/p3bot/agentdex v1.0.0
```

## CLI

Noun groups (`agents`, `models`, `providers`, each aliased to its singular) with shared verbs `list` and `get`.

```
agentdex agents list [filter]     catalogued agents with local detection
agentdex agents get <id>          detail for one agent (aliases: view, show)
agentdex models list [filter]     models across providers (newest release first)
agentdex models get <id>          one model by provider-id/model-id
agentdex providers list [filter]  models.dev providers and API-key env status
agentdex providers get <id>       detail for one provider
agentdex refresh [target]         force refresh: catalog | models.dev | all
agentdex version
agentdex completion               shell completion script
```

### Quickstart

```bash
agentdex agents list
```

```
ID           NAME                VERSION  PROVIDERS       MODELS  BIN
claude-code  Claude Code         2.1.220  anthropic       13      /usr/local/bin/claude
codex        Codex CLI           -        openai          47      missing
opencode     opencode            -        -               -       missing
```

Installed agents lead; other rows show `missing` when the binary is not on `PATH`. `--installed` keeps only detected agents. Agnostic agents (no home provider) show `-` under providers/models unless you pass `--provider`.

```bash
agentdex agents get claude-code
```

```
Agent
id                claude-code
name              Claude Code
version           2.1.220
bin               /usr/local/bin/claude (found)
config_dir        /home/you/.claude
config_local_dir  /home/you/project/.claude
skills_dir        /home/you/.claude/skills
skills_local_dir  /home/you/project/.claude/skills
providers         anthropic
homepage          https://github.com/anthropics/claude-code

Skills
  global
    native  /home/you/.claude/skills (exists)
  local
    native  /home/you/project/.claude/skills (missing)

Provider env
  ANTHROPIC_API_KEY (unset)
```

Models on `agents get` are off by default; pass `--models` (or include `models` in `--fields`) to fill them.

```bash
agentdex providers list
agentdex providers get anthropic --models
agentdex models list --provider anthropic
agentdex models list --agent claude-code
agentdex models get anthropic/claude-sonnet-4-5
```

A provider-agnostic `--agent` on `models list` also requires `--provider`.

### Flags

Global:

| Flag | Effect |
|---|---|
| `--json` | JSON envelope on stdout (see below) |
| `--color auto\|always\|never` | Table colour (default `auto`) |
| `--verbose` / `--quiet` | More or less text detail |
| `--debug` | Diagnostic logging on stderr |

On every `list` and `get`:

| Flag | Effect |
|---|---|
| `--fields` | Select output fields (csv) |

On every `list`:

| Flag | Effect |
|---|---|
| `--order-by` | Sort by field (`models list` default: newest release; others: `id`) |
| `--reverse` | Flip sort direction |

On `agents`:

| Flag | Effect |
|---|---|
| `--installed` | `list`: only agents detected on this machine |
| `--provider` | models.dev provider ids for agnostic agents (repeatable or csv) |
| `--models` | `get`: fill the per-model list |
| `--search-dir` | Extra binary search locations (repeatable) |
| `--bin-path id=path` | Override an agent's binary path (repeatable) |

On `models list`:

| Flag | Effect |
|---|---|
| `--provider` | Scope to models.dev provider ids |
| `--agent` | Scope to a catalogued agent's providers |

On `providers get`:

| Flag | Effect |
|---|---|
| `--models` | Fill the per-model table |

### JSON envelope

```bash
agentdex --json agents list --installed
```

```json
{
  "status": "ok",
  "data": [
    {
      "id": "claude-code",
      "found": true
    }
  ],
  "warnings": []
}
```

Each element of `data` carries the selected fields for that command (more than `id` and `found` by default). On failure: `"status": "error"` with `"error"` set; `warnings` may still be present. `data` is omitted or empty as appropriate.

### Exit codes

| Code | Meaning |
|---|---|
| 0 | Success |
| 1 | Failure |
| 2 | Usage |
| 3 | Not found |
| 4 | Permission |
| 75 | Transient (catalog or models.dev unavailable) |
| 78 | Config (invalid config.cue, invalid catalog, models.dev schema drift) |

### CLI configuration

Optional and **CLI-only**. Library callers use `Open` options, not this file.

Path: `$XDG_CONFIG_HOME/agentdex/config.cue` (fallback `~/.config/agentdex/config.cue`). Absent file means defaults. Flags override config on collision.

```cue
cache_ttl?: string // fallback TTL when section ttl omitted

catalog: {
	module: string | *"github.com/p3bot/agentdex/catalog@v1"
	dir?:   string // local CUE module (no registry; never stale)
	ttl?:   string // version-resolution TTL (default 24h)
}

models: {
	url?: string
	ttl?: string // default 24h
}

search_dirs?: [...string]
bin_paths?: [string]: string
color: "auto" | "always" | "never" | *"auto"
```

Use `catalog.dir` to load an unpublished working-tree catalog while adding an agent.

## Go module

Depend on v1.0.0 as under [Install](#library).

### Example

`Open` does no network I/O. Catalogs resolve lazily on first use, once, under a guard. Safe for concurrent use.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/p3bot/agentdex"
)

func main() {
	ctx := context.Background()

	idx, err := agentdex.Open()
	if err != nil {
		log.Fatal(err)
	}

	res, err := idx.Agents.List(ctx, agentdex.AgentQuery{
		Enrich: agentdex.EnrichCount,
	})
	if err != nil {
		log.Fatal(err)
	}
	for _, w := range res.Warnings {
		fmt.Fprintln(os.Stderr, "warning:", w.Msg)
	}
	for _, a := range res.Items {
		fmt.Printf("%-14s installed=%t models=%d\n", a.ID, a.Detection.Found, a.ModelCount)
	}
}
```

```go
type Index struct {
	Agents    AgentService
	Providers ProviderService
	Models    ModelService
}
```

Each service has `List` → `Result[T]` (items + warnings) and `Get` (exact). Detection is on `Agent.Detection`, not a top-level verb. Cache ops on `Index`: `Refresh`, `CatalogInfo`, `CatalogStale`, `ModelsStale`.

### Options

Common options for `Open` (full list in [package docs](https://pkg.go.dev/github.com/p3bot/agentdex@v1.0.0)):

| Option | Role |
|---|---|
| `WithCatalogDir` | Local CUE module (no network; never stale) |
| `WithCatalogModule` | Override registry module path (exclusive with dir) |
| `WithCacheDir` | Catalog + models.dev cache directory |
| `WithWorkingDir` | Base for relative local paths |
| `WithLookPath` / `WithSearchDirs` / `WithBinPaths` | Binary discovery |
| `WithEnvLookup` | Env for path expansion and provider-env presence only |
| `WithModelsURL` / `WithModelsTTL` / `WithCatalogTTL` | Sources and TTLs |
| `WithHTTPClient` / `WithLogger` | HTTP and structured logging (silent by default) |

### Agents

```go
res, err := idx.Agents.List(ctx, agentdex.AgentQuery{
	Installed: true,
	Providers: []string{"openai"}, // enrichment set for agnostic rows
	Enrich:    agentdex.EnrichNone,
})

detail, err := idx.Agents.Get(ctx, "claude-code", agentdex.AgentGetQuery{
	Enrich: agentdex.EnrichFull,
})
// detail.Detection.Config, detail.Detection.Skills, detail.Coverage, …
```

`Enrich` is the demand axis (each level is a superset). Installation does not gate enrichment.

| Level | Attaches |
|---|---|
| `EnrichNone` | Catalog + detection. `Get` never contacts models.dev; `List` only to validate a non-empty `Providers` filter |
| `EnrichProviders` | Resolved provider set |
| `EnrichCount` | Provider-env presence, model count, coverage on `Get` |
| `EnrichFull` | Full models list |

`Agent.Enrichment` records applied, not-requested, not-applicable (agnostic with no providers), or degraded.

Detection always resolves config and skills paths whether or not the binary is installed. `Found` gates only `BinaryPath` and `Version`.

Skills are classified per scope (global, local): `agents` (shared `.agents` roots), `native` (product tree), `alternatives` (priority order). Primary is derived: agents, else native, else `alternatives[0]`. Full layout on `Detection.Skills` (path + exists per role). Zero `SkillsPaths` means no skills concept.

Agnostic agents have no home provider. Supply models.dev ids via `Providers` on the query (CLI: `--provider`). Home-provider agents reject an explicit set (`ErrProvidersNotAllowed`).

### Providers

```go
pres, err := idx.Providers.List(ctx, agentdex.ProviderQuery{Filter: "anthropic"})
p, err := idx.Providers.Get(ctx, "anthropic")
// p.Env, p.EnvPresent, len(p.Models)
```

From models.dev: id, name, API-key env names, and whether those variables are set (presence only).

### Models

```go
mres, err := idx.Models.List(ctx, agentdex.ModelQuery{
	Scope: agentdex.ModelScope{Providers: []string{"anthropic"}},
})
// Scope.Agent: "claude-code" uses that agent's providers; empty scope = all providers

m, err := idx.Models.Get(ctx, "anthropic/claude-sonnet-4-5")
```

Composite id is `provider-id/model-id` (split on the first slash). Agnostic agent scope without providers → `ErrProvidersRequired`.

### Refresh and staleness

```go
refreshed, err := idx.Refresh(ctx, agentdex.TargetAll) // TargetCatalog, TargetModels
info, err := idx.CatalogInfo(ctx)
stale, err := idx.CatalogStale(ctx)
mstale, err := idx.ModelsStale(ctx)
```

`WithCatalogDir` has nothing to re-resolve (not refreshed, no error).

### Errors and warnings

Match with `errors.Is`. Common sentinels:

| Sentinel | Meaning |
|---|---|
| `ErrCatalogUnavailable` | Cold offline, no prior catalog resolution |
| `ErrCatalogInvalid` | Schema evaluation failed |
| `ErrModelsUnavailable` | models.dev down on Providers/Models (agent ops degrade) |
| `ErrModelsSchema` | Unrecognised models.dev shape (alias of `modelsdev.ErrModelsSchema`) |
| `ErrAgentUnknown` / `ErrNotFound` | Unknown agent, provider, or model |
| `ErrUnknownProvider` | Caller provider id not in models.dev |
| `ErrProvidersRequired` / `ErrProvidersNotAllowed` | Agnostic vs home-provider provider-set rules |
| `ErrMalformedModelID` | Model id with no `/` |

Warnings carry `Kind` (branch) and `Msg` (emit verbatim). Valid on success and error returns; read `Items` only when `err == nil`.

Full list and types: [pkg.go.dev/github.com/p3bot/agentdex@v1.0.0](https://pkg.go.dev/github.com/p3bot/agentdex@v1.0.0).

### models.dev client package

Models.dev only (no agent index):

```go
import "github.com/p3bot/agentdex/modelsdev"
```

Fetches `catalog.json`, merges provider and agnostic maps, checks gross schema drift, caches with stale-on-failure. Imports no agentdex internals. Docs: [pkg.go.dev/github.com/p3bot/agentdex/modelsdev@v1.0.0](https://pkg.go.dev/github.com/p3bot/agentdex/modelsdev@v1.0.0).

## Catalog and caching

| Module | Path | Role |
|---|---|---|
| Go | `github.com/p3bot/agentdex` | Library and CLI |
| CUE | `github.com/p3bot/agentdex/catalog@v1` | `#KnownAgent` schema + agent data |

Published independently. The Go build ignores `catalog/`. The CUE module ships its own `schema.cue`; the loader validates by evaluation. Fetch uses `cuelang.org/go/mod/modconfig` and honours `CUE_REGISTRY` and `cue login` (no agentdex-specific auth).

Caches live under `$XDG_CACHE_HOME/agentdex/` (default TTL 24h): catalog **version resolution** over CUE's content cache, plus models.dev `catalog.json`.

- Latest-version resolution needs network; a pinned `module@version` can be served offline from CUE's content cache.
- After TTL expiry, failed re-resolution keeps the last version and reports stale (usable with a warning).
- First run with no network and no prior resolution → `ErrCatalogUnavailable` (CLI exit 75). Accepted behaviour.
- Unreachable models.dev: agent ops degrade; `Providers` / `Models` return `ErrModelsUnavailable`.

Force a refresh: `agentdex refresh` or `Index.Refresh`.

## Platforms

- Linux, macOS, and WSL (agents installed natively in WSL Linux).
- No native Windows; no Windows-host agents via WSL PATH interop.
- Pure Go (`CGO_ENABLED=0`).

## Contributing

Adding an agent is a catalog edit, not a code change: research outside facts, edit `catalog/agents.cue`, `cue vet`, exercise with `catalog.dir`, publish a new catalog version. Workflow and schema: [AGENTS.md](AGENTS.md). Skills matrix: [docs/agents-skills-path-matrix.md](docs/agents-skills-path-matrix.md).

Library and CLI changes: open an issue or pull request against this repository. Prefer the standard library and existing dependencies; keep the binary pure Go.

## License

[MPL-2.0](LICENSE).
