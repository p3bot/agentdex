---
id: ad-n779
status: in-progress
order: "a1"
tags: [catalog, agents, interactive]
created: "2026-08-02T17:48:44+10:00"
summary: Add top terminal AI coding agents to the catalog one at a time, following the AGENTS.md workflow, interactively
---
# Catalog coding agents

## Goal

Expand the agentdex catalog with major terminal AI coding agents, one at a time. Interactive: research, propose, confirm, exercise, land. Procedure is `AGENTS.md` "Adding an agent to the catalog" — do not restate it here.

## Scope

In: additive `#KnownAgent` entries, path matrix when skills apply, local validate/exercise, catalog-only `@v1` publish on user request.

Out: schema breaks without a paired binary release; Go/detection special-cases; agent-internal config; IDE-only products; retired products (Gemini CLI → `agy`); non-coding name collisions; native Windows / WSL-host PATH interop agents.

## Current State

Registry still **catalog@v1.4.0**. Tree ahead of registry with three exercised unpublished entries (`crush` and `kimi-code` already on `main` via `fcff7cd`; `qwen-code` applied this cycle, not committed yet):

| id | bin | provider / agnostic | status |
|---|---|---|---|
| `crush` | `crush` | `agnostic: true` | entry + matrix + context; exercised; on main |
| `kimi-code` | `kimi` | `provider: ["kimi-for-coding"]` | entry + matrix + context; exercised; on main |
| `qwen-code` | `qwen` | `agnostic: true` | entry + matrix + context; `cue vet` OK; exercised via `catalog.dir` |

Full keys in tree: `agy`, `aider`, `augment`, `claude-code`, `cline`, `codewhale`, `codex`, `copilot`, `crush`, `goose`, `grok`, `kimi-code`, `kiro`, `opencode`, `qwen-code`.

Session context clones (tracked indexes/repos.csv; trees gitignored): `charmbracelet-crush`, `moonshotai-kimi-code`, `qwenlm-qwen-code`.

Next cycle: **OpenHands** (`openhands`).

`qwen-code` uncommitted (host git). Publish only on request; prefer one session batch of exercised entries.

Exercise unpublished work with `catalog.dir` under a temp `XDG_CONFIG_HOME` (see `AGENTS.md`). Repo constraints: Go 1.25 pure Go, CUE `v0.16.0`, Linux/macOS/WSL.

## References

- models.dev (provider join keys): https://models.dev/ and https://models.dev/api.json
- Market lists that seeded earlier queue work: MorphLLM, Codersera 2026, CodePick terminal CLI comparison (mid-2026)

## Requirements

1. Work the pending queue in order, one agent per interactive cycle.
2. Follow `AGENTS.md` add-agent procedure end-to-end; publish only when the user asks.
3. Confirm outside facts (binary, docs, OSS clone under `.agents/context/` when applicable, models.dev providers). No agent-internal config.
4. Kebab-case map key is the sole id. Keep this queue and outcome log current.
5. Additive `@v1` publishes only after local exercise of included entries.

### Work queue (pending)

Process top to bottom. Skip only with a reason in the outcome log.

| Order | Candidate | Suggested id | Notes |
|---|---|---|---|
| 1 | OpenHands | `openhands` | bin openhands |
| 2 | Amp | `amp` | Sourcegraph; bin amp |
| 3 | Continue CLI | `continue` | bin cn |
| 4 | Kilo Code CLI | `kilo` | Confirm bin |
| 5 | Pi | `pi` | Minimal harness |
| 6 | Plandex | `plandex` | Large multi-file tasks |
| 7 | Open Interpreter | `open-interpreter` | bin interpreter |
| 8 | ForgeCode | `forge` | Confirm bin |
| 9 | Hermes Agent | `hermes` | Nous Research CLI/TUI |
| 10 | OpenClaw | `openclaw` | Personal assistant / gateway |

Do not re-add agents already correct in the catalog unless research finds a factual error.

Skipped (not catalogued): Zapier; Gemini CLI (use `agy`); Roo Code (IDE-first); OpenRouter model names as agents (DeepSeek, MiMo, Hy3, GLM, Nemotron, MiniMax, Step). Deferred: Cursor agent CLI, Windsurf agent CLI, Claw Code, SWE-agent, Warp, Tabby.

## Constraints

- Follow `AGENTS.md`. Catalog-only additive work. CUE pin `v0.16.0`.
- Interactive: user confirms each proposed entry; user requests any registry publish.
- Scoped Commits. Prefer batch publish per session, not per agent.

## Implementation Plan

One agent per cycle. Do not batch silent multi-agent passes.

1. Orient: this project, `AGENTS.md` add-agent, `catalog/agents.cue`, path matrix. Confirm next queue item with user.
2. Research outside facts; OSS → `.agents/context/` per that guide. Propose entry; wait for confirmation.
3. Apply entry (+ path matrix if skills).
4. `cue vet ./...` and `cue mod tidy` from `catalog/`.
5. Exercise via `catalog.dir`; `agents list` / `agents get <id>`; confirm with user.
6. Land on approval (commit or multi-agent commit per user). Publish only if user asks now.
7. Outcome log; next queue item. Stop when queue empty or user ends.
8. Publish (user-driven): one `@v1` with all exercised-unpublished entries; log it.

Out of scope or weak facts → skip with reason; do not force a weak entry.

### Outcome log

| Batch / agent | Result | Notes |
|---|---|---|
| catalog@v1.3.0 | published | `grok`, `codex`, `copilot` |
| catalog@v1.4.0 | published | `aider`, `goose`, `kiro`, `augment`, `cline`, `codewhale` |
| DeepSeek TUI | skipped | rebranded → `codewhale` |
| crush | exercised | agnostic; on main; unpublished pending session batch |
| kimi-code | exercised | bin `kimi`; provider `kimi-for-coding`; on main; unpublished |
| qwen-code | exercised | bin `qwen`; agnostic; entry + matrix + context; uncommitted |

Append one short row per later agent or registry publish.

## Implementation Guidance

- Prefer real binary when practical; else OSS clone + docs required.
- Context clones: project-local `.agents/context/` (gitignored trees; tracked indexes/repos.csv).
- Docs vs source: trust source; note discrepancies in the research proposal.
- Provider-agnostic → `agnostic: true`, no `provider`.
- Outcome log: result + one-line notes only. Leave tree clean between agents.
- Publish cadence: session batch or on request; installs re-resolve after catalog TTL.

## Acceptance Criteria

- Each queue item catalogued (entry + matrix if needed + validated + exercised), skipped with reason, or still pending in order.
- No entry without `AGENTS.md` procedure and user field confirmation.
- `cue vet` / `cue mod tidy` clean after each apply; `catalog.dir` + `agents get` before any publish inclusion.
- User-requested registry releases include all then-pending exercised entries and appear in the outcome log.
- No schema-breaking catalog-only publish.