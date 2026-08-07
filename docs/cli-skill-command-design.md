# CLI skill command design

A reusable design for any CLI that ships an agent skill and wants
install / list / uninstall against real coding-agent skills directories,
without hardcoding per-product paths.

agentdex supplies path authority (catalog + detection). The CLI owns
skill content, the skill id, filesystem writes, and command UX.

This document is the design contract. It is not an agentdex API change:
callers use the existing library (`Open`, `Agents.List` / `Get`,
`EnrichNone`, `Detection.Skills`).

## Goals

- One skill package per CLI, discoverable by agents that load skills from
  known directory trees.
- User-initiated install only; never automatic on unrelated commands
  (init, first run, etc.).
- No hardcoded agent skill roots in the CLI; paths come from agentdex.
- Shared multi-tenant roots (for example `~/.agents/skills`) handled
  honestly: path operations, not fake per-agent isolation.
- List and uninstall operate on what is on disk under agentdex-resolved
  candidates; no private install ledger.
- Offline / catalog failure fails closed and points the user at printing
  the skill for manual placement.

## Non-goals

- Reading or interpreting an agent's internal configuration (agentdex
  boundary).
- Isolating skills per agent inside a shared agents root (product
  limitation of those agents, not this design).
- Installing arbitrary third-party skills (one product skill id).
- models.dev enrichment (use `EnrichNone` only).
- Defining how agents discover or invoke skills beyond writing the
  conventional layout.

## Terms

| Term | Meaning |
|---|---|
| CLI | The product binary (for example `pj`, `start`, `snag`) |
| skill id | Stable kebab-or-simple name: directory segment and frontmatter `name` (for example `pj`) |
| skill file | `<skills_root>/<skill-id>/SKILL.md` |
| skill body | Full file contents the CLI embeds or generates (YAML frontmatter + markdown) |
| catalog agent | One entry in the agentdex agent catalog |
| Found | agentdex detection: agent binary present (PATH / overrides) |
| skills concept | Catalog entry defines at least one skills scope/role |
| Primary | Derived install/query target: agents role, else native, else alternatives[0] |
| Native | Product-private skills root from the catalog |
| Shared | Catalog agents role path (for example `~/.agents/skills`) |
| candidates(a) | Unique non-empty absolute paths among Primary(a), Native(a), Shared(a) at a scope |
| scope | global (user-wide) or local (project / working directory) |
| default agent set | Found && has skills concept |
| S | Agent set for this invocation (default set or explicit ids) |
| R | Default agent set minus S (remaining installed sharers); used only for uninstall blockers |

Primary, Native, Shared, and path expansion (including `~` and relative
local roots) are defined by agentdex. Do not re-derive primary in the CLI.

## Dependency

- Go module: `github.com/p3bot/agentdex` (use a published tag, for example
  `v0.0.2` or later).
- Construct `Index` with `agentdex.Open`, inject `WithWorkingDir` for local
  roots and test seams (`WithLookPath`, `WithEnvLookup`, `WithCatalogDir`)
  as needed.
- Agent operations: `EnrichNone` (catalog + detection only).
- Map library errors with `errors.Is` against package sentinels
  (`ErrCatalogUnavailable`, `ErrCatalogInvalid`, `ErrAgentUnknown`, …).

## Skill identity

Each CLI chooses one skill id for life of the product surface.

| Concern | Rule |
|---|---|
| Directory | `<skills_root>/<skill-id>/` |
| File name | `SKILL.md` (ecosystem convention) |
| Frontmatter | `name: <skill-id>` must match the directory segment |
| Body | CLI-defined; install writes the current body; uninstall does not require byte equality with that body |

Example for a CLI named pj: skill id `pj`, path `…/pj/SKILL.md`,
frontmatter `name: pj`.

## CLI surface

Suggested verbs (names may match the product; semantics should not drift):

```text
<cli> skill                         # print skill body to stdout (no agentdex required)
<cli> skill install [agents...] [--local]
<cli> skill uninstall [agents...] [--local]
<cli> skill list [--local]
```

Print (`skill` with no subcommand) is independent of agentdex so a cold
catalog never blocks manual bootstrap.

### Multi-value agents

Optional agent ids are space-separated positionals, not CSV and not a
required repeated flag:

```text
<cli> skill install
<cli> skill install grok claude-code
```

### Location flag

| Flags | Skills scope |
|---|---|
| default | global only |
| `--local` | local (project) only |

No dual-write flag. Both scopes requires two invocations. Do not name this
flag with product-specific words that collide (for example avoid "scope" if
the CLI already uses "scope" for something else).

## Default agent set

```text
default set = catalog agents where Detection.Found
              and the agent has a skills concept
```

Empty default set:

| Verb | Behaviour |
|---|---|
| install / uninstall | Fail (usage-class): nothing to target |
| list | Empty inventory, exit 0 |

## Explicit agent ids

Allowed on install and uninstall only (not list).

- Resolve each id with agentdex; unknown id fails.
- No skills concept fails (do not invent paths).
- Found is not required: paths still resolve; supports uninstall after
  the binary is gone and pre-seed before install.
- Named install path rule still requires a writable path (see Install).

## Install

### Path rule

| Args | Path per agent in S |
|---|---|
| none | Primary |
| one or more ids | Native if non-empty, else Shared (agents role) |

If a named agent has neither native nor shared, fail that id (no writable
path under the named rule).

De-dupe by cleaned absolute path; write once per path.

### Write

1. Destination: `<path>/<skill-id>/SKILL.md`
2. Create `<path>/<skill-id>/` as needed (`MkdirAll`)
3. Write the current skill body (overwrite if present)
4. Hard fail if `path` exists and is not a directory, or create/write fails

### Rationale (two planes)

- No-args Primary arms every installed agent at its preferred root (shared
  agents tree when that is primary; native-only agents such as products that
  only document `~/.claude/skills` still get a write).
- Named Native-else-Shared prefers product-private trees when they exist;
  agents that only support the shared agents directory still install there.

## Uninstall

### Path universe (S3)

Uninstall is not limited to the install plane of the same argv shape. For
each agent in S, consider every location install could have used:

```text
candidates(a) = unique non-empty absolute paths among:
  Primary(a), Native(a), Shared(a)
```

at the selected location (global or local).

```text
paths = union of candidates(a) for a in S
R     = default agent set \ S
```

### Blockers (multi-tenant paths)

For each path P in paths:

```text
blockers = { b in R | P is in candidates(b) }
```

If blockers is non-empty, keep any skill install at P and report who
still uses it. Not an error by itself.

If blockers is empty, attempt removal under the purity rules below.

This applies to any path, including shared `…/agents/skills`, without a
separate special case.

### Removal purity

Skill directory: `D = P/<skill-id>/`.

| Condition | Action |
|---|---|
| D does not exist | absent; OK |
| D exists but is not a directory | hard fail for that path |
| Directory entries are not exactly `{SKILL.md}` | keep; report extras (or missing skill file) |
| `SKILL.md` frontmatter `name` is not skill id | keep; report not ours |
| All checks pass | remove entire directory D (`RemoveAll`); do not remove P |

Uninstall does not require the file body to match the CLI's current
embedded skill text. Edited copies with the correct skill id still remove.
Foreign files or a wrong `name` leave the tree untouched.

### Explicit ids and R

Not-installed agents may appear in S and contribute candidates; they never
appear in R. Full uninstall (S = default set) has R empty, so every
unblocked candidate path is eligible for purity checks and removal.

## List

```text
<cli> skill list [--local]
```

- Same agent set as install/uninstall with no args (default set only).
- No agent positionals: inventory is the whole default set for that location.
- Same candidates union as uninstall; emit a row only when
  `…/<skill-id>/SKILL.md` exists.
- De-dupe by path; a row may list which agents in the set claim that path.
- Orphans under agents no longer Found are not listed (lint on disk is
  acceptable; explicit uninstall id can still GC if desired).
- Empty default set: empty list, exit 0.

Optional later: an `in_sync` column (body equals current skill text). Not
required for install/uninstall correctness.

## Catalog and offline errors

agentdex catalog is required for install, list, and uninstall (path
resolution). Print-only `skill` is not.

| Failure | CLI behaviour |
|---|---|
| Catalog unavailable / invalid | Fail closed; no hardcoded fallback paths; instruct the user to print the skill (`<cli> skill`) and place it manually in their agent's skills directory; optional note that first catalog resolve needs network then cache |
| Unknown agent id | Usage-class failure |
| No skills concept / no writable named path | Usage-class failure |
| Empty default set on install/uninstall | Usage-class failure |
| I/O after paths resolved | Ordinary failure with path context |

Do not suggest invented paths in error text.

## What the CLI implements vs agentdex

| Concern | Owner |
|---|---|
| Agent catalog, path templates, expansion, Found, Primary | agentdex |
| Skill id, skill body, embed/print | CLI |
| Mkdir, write, read frontmatter name, RemoveAll | CLI |
| Default set filter, path rules, blockers R, purity | CLI (this design) |
| models.dev / providers / models | unused for these verbs |

## Implementation sketch (library use)

```text
idx, err := agentdex.Open(agentdex.WithWorkingDir(cwd), ...)
res, err := idx.Agents.List(ctx, agentdex.AgentQuery{
    Installed: true, // when building the default set from List
    Enrich:    agentdex.EnrichNone,
})
// or Get per explicit id with EnrichNone

// Skills paths: agent.Detection.Skills.Global|Local
//   .Primary, .Native, .Agents  (PathEntry.Path / Exists)
// Primary derivation is already applied on Primary.
```

Prefer `List` + filter for the default set, `Get` for explicit ids.
Working directory must match the process cwd (or the CLI's notion of
project root) so local paths align with agentdex.

## Testing seams

- `WithCatalogDir` for a fixture catalog (no registry).
- `WithLookPath` / `WithBinPaths` to control Found without host PATH.
- `WithEnvLookup` + temp home for `~` expansion.
- `WithWorkingDir` temp dir for local scope.
- Assert writes only under expanded candidate paths; assert uninstall
  purity keeps foreign files and wrong `name`.

## Product checklist

When adopting this design in a CLI:

1. Choose skill id; embed `SKILL.md` with matching `name`.
2. Implement print verb without agentdex.
3. Depend on a published agentdex module version.
4. Implement install / uninstall / list per this document.
5. Wire location flag and positional agents as specified.
6. Map catalog errors to stable messages with manual-install guidance.
7. Document that install is user-initiated and never implied by unrelated commands.

## Design status

Path and verb semantics were worked out for a concrete CLI (pj) against
agentdex v0.0.2 and the classified skills roles (agents / native /
alternatives / primary). This document generalises that contract for any
CLI that ships one agent skill and uses agentdex for directory discovery.
