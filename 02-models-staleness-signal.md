# Models.dev Staleness Signal

## Goal

Make agentdex report when the models.dev catalog it is serving is a stale
fallback, the same way it already reports a stale agent catalog. Today the agent
catalog surfaces staleness through both a warning and a query, while models.dev
serves stale-on-failure data silently, so a caller cannot tell fresh models.dev
data from a stale copy served after a failed refetch. This closes the one place
in the library where degraded data reaches a caller with no signal.

## Scope

In scope:

- A staleness signal exposed from the `modelsdev` package client.
- A new `WarnModelsStale` warning kind in the root package, emitted by the
  operations that serve models.dev-derived data and already carry a warnings
  channel.
- A new `Index.ModelsStale` query mirroring the existing `Index.CatalogStale`,
  so operations with no warnings channel still have a way to ask.
- CLI presentation of those warnings on `providers list`, which today discards
  the library warnings channel; agents and models already forward it.

Out of scope:

- Any change to when models.dev is fetched, cached, or falls back. The
  stale-on-failure behaviour stays exactly as it is; only its visibility
  changes.
- Any change to the agent-catalog staleness path. It is the model to copy, not
  to modify.
- Retry, background refresh, or configurable staleness policy.
- CLI changes beyond forwarding library warnings from `providers list`.

## Current State

The library is a Go module at `github.com/start-cli/agentdex` with a public
root package (`agentdex`) and a public leaf package (`modelsdev`). The leaf
imports nothing from the root and is independently consumable.

How the agent catalog already surfaces staleness, the pattern to mirror:

- `core` (core.go) holds a `catStale bool` alongside the cached catalog.
  `resolveCatalog` sets it when the loader reports the last resolved version was
  reused after a failed re-resolution.
- Every catalog-resolving operation that carries a warnings slice appends
  `staleWarning()` when the flag is set: `Agents.List` (agents.go), `Agents.Get`
  via `AgentDetail.Warnings`, and `Models.List` when scoped to an agent
  (models.go). The wording lives in one shared constructor, `staleWarning()`
  (agents.go), so it never drifts between call sites.
- `Index.CatalogStale(ctx)` (index.go) resolves the catalog lazily, then reports
  the flag. On a cold-offline first call with nothing cached it returns
  `ErrCatalogUnavailable` rather than a misleading `false`.
- The warning kind `WarnStaleCatalog` is one of the `WarningKind` constants in
  enrich.go, and each `WarningKind` has a `String()` case.

How models.dev is served, where staleness is known but not exposed:

- `modelsdev.Client.fetchMerge` (modelsdev/client.go) produces a usable catalog
  from one of four paths: a within-TTL cache hit, a fresh network fetch, or,
  when the fetch fails, a stale copy read back from the on-disk cache. Only that
  last path is stale. It returns an error only when the fetch fails and there is
  no cache to fall back on.
- The stale-fallback branch is the block that re-decodes `cachedData` after the
  network `get` failed. That branch is the single point where the client knows
  it is serving stale data.
- The merged catalog is memoised for the client's lifetime through `load`,
  `runFetch`, and `afterInflight`; the client never re-merges. A refresh is
  picked up by constructing a fresh client, which `core.refreshModels` does with
  a force-refresh client that it swaps in under a guard.
- `core.modelsClient()` constructs one shared client lazily and reuses it, so a
  single fetch backs the concurrent detection fan-out. `core` reads models.dev
  through this one client.
- The leaf's normal serve methods (`Catalog`, `Models`, `Provider`) return no
  fresh-versus-stale signal. `WithForceRefresh` is the only honest mode today,
  and it changes fetch behaviour rather than reporting on a normal serve.

Consequence: `core` has no bit to read and emits no warning, so a stale
models.dev serve is invisible to every caller.

CLI presentation today:

- `agents list` / `agents get` and `models list` already pass library warnings
  into the envelope via `libWarnings`.
- `providers list` (`internal/cli/providers.go`) calls `a.ok` with a nil warnings
  slice and documents that a provider listing raises no warnings. Once
  `Providers.List` emits `WarnModelsStale`, that path would still hide it.

## Requirements

1. The `modelsdev.Client` exposes whether its memoised catalog was served from
   the stale-fallback path. The signal reflects the single load the client
   memoises. It reads meaningfully only after the client has loaded; a caller
   that needs it after a bare load must be able to trigger the load first, the
   same way `Index.CatalogStale` triggers catalog resolution.

2. The root package defines `WarnModelsStale` as a new `WarningKind`, with its
   `String()` case, and a single shared constructor for its wording, matching
   how `WarnStaleCatalog` and `staleWarning()` are structured.

3. Every operation that serves models.dev-derived data and carries a warnings
   channel appends `WarnModelsStale` when, and only when, that operation
   actually consulted models.dev and the served copy was a stale fallback. This
   covers `Agents.List` and `Agents.Get` at the enrichment levels that reach
   models.dev, `Providers.List`, and `Models.List`. An operation that did not
   touch models.dev must not warn about its staleness.

4. `Index.ModelsStale(ctx)` reports whether the models.dev catalog currently
   served is a stale fallback. It mirrors `Index.CatalogStale`: it triggers the
   models.dev load lazily, returns `ErrModelsUnavailable` on a cold-offline
   first call with nothing cached, and otherwise returns the staleness bit.

5. The operations that return a bare value with no warnings channel
   (`Providers.Get`, `Models.Get`) are left without a warning; their callers use
   `Index.ModelsStale` when they need the signal. Do not widen those return
   types to carry a warning.

6. Doc comments that currently state an operation raises no warnings are
   corrected where requirement 3 adds one. In particular, `Providers.List`
   documents that it now raises `WarnModelsStale` on a stale models.dev serve,
   while still raising no agent-catalog warning.

7. The CLI `providers list` command forwards library warnings into the JSON
   envelope and text stderr the same way agents and models list do
   (`libWarnings(res.Warnings)` into `a.ok` / `a.fail`). Its comment that a
   provider listing raises no warnings is corrected.

## Constraints

- Go 1.25, pure Go, builds with `CGO_ENABLED=0`. No new module dependency; the
  mechanism already exists and only needs to be threaded and exposed.
- The `modelsdev` package stays a leaf: it must not import the root package, the
  agent catalog, or any `internal` package. The staleness signal is expressed in
  terms the leaf already owns.
- The stale-fallback serving behaviour must not change. A caller that ignores
  the new signal sees identical data and identical errors to today.
- Exported additions carry godoc in the surrounding form. Match the existing
  code's conventions for naming, guarding shared state, and structured warnings.

## Implementation Plan

1. Thread the stale-fallback outcome out of `fetchMerge` and record it on the
   client alongside the memoised catalog, set under the same guard that
   memoises the catalog in `runFetch`, and readable by waiters through
   `afterInflight`. A within-TTL cache hit and a fresh fetch are not stale; only
   the post-failure cache re-decode is.

2. Expose the signal as a public accessor on `modelsdev.Client` that reflects
   the memoised load. Give it godoc that states it is meaningful only after a
   load and names the stale-fallback condition it reports.

3. In `core`, after resolving models.dev through the shared client, read the
   client's staleness and make it available to the operation layer, mirroring
   how `catStale` is read for the catalog. Prefer reading the client directly
   over caching a second copy of the bit, since the client already owns it.

4. Add `WarnModelsStale` to the `WarningKind` constants and its `String()` case,
   and add a shared constructor for its message next to `staleWarning()`.

5. Append `WarnModelsStale` in the operations named in requirement 3, gated on
   the operation having consulted models.dev and the serve being stale. Reuse
   the shared constructor so the wording lives in one place.

6. Add `Index.ModelsStale(ctx)` next to `Index.CatalogStale`, with the same
   lazy-load-then-report shape and the same cold-offline error contract, using
   `ErrModelsUnavailable`.

7. Correct the affected doc comments (requirement 6).

8. In `internal/cli/providers.go`, forward `libWarnings(res.Warnings)` from
   `Providers.List` into `a.ok` (and `a.fail` on the error path if warnings are
   present), matching agents and models list. Update the providersList comment.

## Implementation Guidance

- The cold-offline contract for `Index.ModelsStale` should fall out of the
  normal load path: trigger the models.dev load, map a fetch-with-no-cache
  failure to `ErrModelsUnavailable` the way other models.dev operations do
  through `mapModelsErr`, and report the bit otherwise. Do not invent a separate
  error path.
- Gate the warning on real consultation by this operation, not on the requested
  enrichment level or on whether the network fetch was the first load. The
  models.dev client loads once and memoises; every later operation that invokes
  it reuses that memoised serve, and a stale fallback must still warn. Concrete
  cases that consult: `Providers.List` and `Models.List` whenever they call the
  client; `Agents.Get` at `EnrichProviders` for an agnostic agent with a provider
  set (id validation), and at `EnrichCount`/`EnrichFull` once a provider set is
  resolved; `Agents.List` whenever a non-empty provider filter is validated
  against models.dev (any enrichment level), and at `EnrichCount`/`EnrichFull`
  for the listing-wide Catalog reachability probe and per-agent enrichment.
  Paths that never invoke the client — `EnrichNone` with no provider filter, a
  home-provider agent at `EnrichProviders`, an agnostic agent with no provider
  set — must not emit `WarnModelsStale`. When the operation does consult and the
  serve is stale, append the warning once at the operation level (including on
  the error path when a usable serve already happened), not per agent row.
- Tests favour real behaviour. `internal/modelsdevtest/server.go` provides a
  models.dev test server; drive staleness by serving once to populate the cache,
  then failing the next fetch so the client falls back, and assert both the
  accessor and the warning. Use `t.TempDir` for the cache directory and
  `t.Setenv` for environment isolation, consistent with the existing tests.

## Acceptance Criteria

1. A `modelsdev.Client` that serves a stale copy after a failed refetch reports
   stale through its accessor; one serving a within-TTL cache hit or a fresh
   fetch reports not-stale.
2. `Agents.List`, `Agents.Get`, `Providers.List`, and `Models.List` include a
   `WarnModelsStale` warning when they consulted models.dev and the serve was a
   stale fallback, and omit it otherwise, including when the operation did not
   reach models.dev at all.
3. `Index.ModelsStale(ctx)` returns true on a stale serve, false on a fresh
   serve, and `ErrModelsUnavailable` on a cold-offline first call with nothing
   cached.
4. `Providers.Get` and `Models.Get` signatures are unchanged.
5. The stale-fallback data returned by every operation is byte-for-byte what it
   returns today; only the accompanying signal is new.
6. `agentdex providers list` surfaces a library `WarnModelsStale` warning in the
   JSON envelope and on text stderr when `Providers.List` returns one; agents
   and models list continue to forward warnings unchanged.
