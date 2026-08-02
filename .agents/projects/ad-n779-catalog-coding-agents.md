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

Expand the agentdex catalog with the major terminal AI coding agents in market use, one agent at a time, so detection and models.dev enrichment cover the tools people actually run. Work is interactive: each agent is researched, proposed, confirmed, exercised, and only then landed.

## Scope

In scope:

- Additive catalog entries under the existing `#KnownAgent` schema in `catalog/agents.cue`.
- Matching rows in `docs/agents-skills-path-matrix.md` when an entry has skills.
- Local validation and exercise against the working-tree catalog before publish.
- Catalog-only publish under the `@v1` major line for additive entries.
- Finishing agents already drafted in the working tree but not yet published.

Out of scope:

- Schema-breaking catalog changes (those need a paired agentdex binary release).
- Go code changes, new detection special-cases, or agent-internal config parsing.
- IDE-only products with no installable terminal agent binary (Cursor editor, Windsurf IDE, Continue as extension-only), unless research finds a clean CLI bin and outside facts.
- Retired products (Gemini CLI is superseded by Antigravity / `agy`; do not add as a current agent).
- Non-coding tools that collide on names (for example AWS Copilot CLI).
- Native Windows agents or Windows-host agents reached only through WSL PATH interop.

## Current State

agentdex loads agents from the CUE module `github.com/start-cli/agentdex/catalog@v1` at runtime. Adding an agent is a catalog edit, not a code change. The add-agent procedure, field rules, skills roles, validation, `catalog.dir` exercise, and publish policy live in repository root `AGENTS.md` under "Adding an agent to the catalog". Follow that section for every agent; this project does not restate it.

Catalog map keys already present in `catalog/agents.cue`:

| id | Status |
|---|---|
| `agy` | In registry (catalog@v1 line) |
| `claude-code` | In registry (catalog@v1 line) |
| `codex` | Added this session; published in catalog@v1.3.0 |
| `copilot` | Added this session; published in catalog@v1.3.0 |
| `grok` | Added this session; published in catalog@v1.3.0 |
| `opencode` | In registry (catalog@v1 line) |

Config for local exercise: `$XDG_CONFIG_HOME/agentdex/config.cue` (fallback `~/.config/agentdex/config.cue`). Use `catalog.dir` pointed at this repository's `catalog/` for unpublished work; prefer a temporary `XDG_CONFIG_HOME` so user config is not overwritten.

Platform and module constraints for this repo are in `AGENTS.md` (Go 1.25, pure Go, CUE `v0.16.0`, Linux/macOS/WSL).

## References

Market and product research that shaped the work queue (mid-2026):

- MorphLLM agent rankings and Terminal-Bench notes: https://www.morphllm.com/ai-coding-agent
- Codersera 2026 agent guide (Cursor, Claude Code, Cline, Aider, OpenCode, Windsurf, Codex, Antigravity, Grok): https://codersera.com/blog/ai-coding-agents-complete-guide-2026/
- CodePick terminal CLI comparison (Claude Code, Gemini CLI, Codex CLI, Kiro, Copilot CLI, Cline, Aider): https://codepick.dev/en/compare/cli-ai-coding-tools-2026/
- Claude Code vs Codex CLI vs Gemini CLI comparisons: https://dev.to/rahulxsingh/claude-code-vs-codex-cli-vs-gemini-cli-which-ai-terminal-agent-wins-in-2026-55f5 and https://www.deployhq.com/blog/comparing-claude-code-openai-codex-and-google-gemini-cli-which-ai-coding-assistant-is-right-for-your-deployment-workflow
- OpenAI Codex CLI: https://github.com/openai/codex
- GitHub Copilot CLI: https://github.com/features/copilot/cli
- models.dev provider catalog (join keys for `provider`): https://models.dev/ and https://models.dev/api.json
- Antigravity / Gemini CLI succession (agy already catalogued): product migration notes mid-2026

Session research notes:

- Grok: installed binary `grok`; docs under `~/.grok/docs/`; models.dev provider `xai`
- Codex: binary `codex` (release probe); config `~/.codex` / `.codex`; models.dev provider `openai`
- Copilot: binary `copilot` (not `gh copilot`); config `~/.copilot` / `.github`; models.dev provider `github-copilot`

## Requirements

1. Work the agent queue below in order, one agent at a time, using the interactive cycle in the Implementation Plan.
2. Every accepted agent is added through the procedure in `AGENTS.md` "Adding an agent to the catalog" (research, entry, path matrix when skills apply, `cue vet` / `cue mod tidy`, `catalog.dir` exercise). Publish is not required at the end of every agent cycle; see Implementation Guidance.
3. Research confirms outside facts against the real product: installed binary when available, product docs, open-source clone and source inspection when the agent is open source, and models.dev for each `provider` id.
4. Entries report only the outside of the agent. No fields that require reading agent internal configuration.
5. Agent ids stay kebab-case; the map key is the sole identity.
6. The work queue and a short per-agent outcome log are kept up to date in this project document as agents complete, skip, or defer.
7. Catalog publishes are additive `@v1` releases only after the included entries have passed local exercise, and only when the user asks for a registry release.

### Work queue

Process top to bottom. Skip only with an explicit reason recorded in the outcome log (retired product, no terminal bin, out of scope, blocked on user decision).

| Order | Candidate | Suggested id | Notes |
|---|---|---|---|
| 1 | Grok (finish) | `grok` | Done; catalog@v1.3.0 |
| 2 | OpenAI Codex CLI | `codex` | Done; catalog@v1.3.0 |
| 3 | GitHub Copilot CLI | `copilot` | Done; catalog@v1.3.0 |
| 4 | Aider | `aider` | Open source; likely agnostic; verify |
| 5 | Goose | `goose` | Block OSS agent harness; confirm bin and outside facts |
| 6 | Amazon Kiro CLI | `kiro` | Confirm bin, install path, provider |
| 7 | Augment CLI | `augment` | Enterprise CLI; confirm bin and outside facts |
| 8 | Cline CLI | `cline` | Confirm a real CLI bin exists beyond the VS Code extension |
| 9 | DeepSeek TUI / related | as research decides | Confirm product name, bin, and fit before treating as next |

Do not re-add agents already correct in the catalog unless research finds a factual error; fix in place if so.

Deferred unless a later cycle promotes them: Cursor agent CLI, Windsurf agent CLI, Continue, Roo Code, Kilo Code, Void — only if a terminal agent binary and clean outside facts exist.

## Constraints

- Follow repository `AGENTS.md` for catalog delivery, caching, schema, platforms, commit style, and the add-agent workflow. Do not invent a parallel procedure.
- Pure catalog work for additive agents: no Go changes, no embedded catalog, no agentdex-specific registry auth.
- CUE language version remains `v0.16.0` as pinned by the catalog module.
- Interactive mode: do not advance past a research proposal without user confirmation for that agent. Do not publish without an explicit user request for a registry release.
- Commit messages use Scoped Commits (`catalog`, `docs`, or as appropriate). Catalog publish is independent of the Go binary for additive entries.
- Do not publish after every agent by default. Batch publishes are preferred within a work session; publish mid-queue only when the user wants something live now.

## Implementation Plan

Each agent is one cycle. Do not batch multiple agents into one silent pass.

1. Orient: read this project, `AGENTS.md` add-agent section, current `catalog/agents.cue`, and the path matrix. Present the next queue item to the user and confirm it is the one to work.
2. Research: gather outside facts per `AGENTS.md` (binary, docs, open-source clone when applicable, models.dev). Present a proposed entry (id, fields, skills roles, open questions) to the user. Wait for confirmation or edits.
3. Apply: write `catalog/agents.cue` and update the path matrix when skills apply.
4. Validate: from `catalog/`, run `cue vet ./...` and `cue mod tidy`.
5. Exercise: load via `catalog.dir` (isolated `XDG_CONFIG_HOME` preferred). Run `agentdex agents list` and `agentdex agents get <id>`. Confirm version, paths, and provider model counts with the user.
6. Land: on user approval of the entry, commit the tree changes for that agent (or leave uncommitted if the user prefers a multi-agent commit). Do not publish here unless the user asks for a registry release now.
7. Advance: mark the queue item done or skipped with reason, then return to step 1 for the next agent. Stop when the queue is exhausted or the user ends the session.
8. Publish (optional, user-driven): when the user wants the registry updated, publish one new catalog `@v1` version that includes all exercised-but-unpublished additive entries (same mechanism as start/library). Record the publish in the outcome log.

If research shows a candidate is out of scope or not ready, record the reason in the outcome log, skip, and continue. Do not force a weak entry.

### Outcome log

Update as work proceeds.

| Agent | Result | Notes |
|---|---|---|
| `grok` | published | catalog@v1.3.0; entry + matrix; exercise passed |
| `codex` | published | catalog@v1.3.0; entry + matrix; provider openai |
| `copilot` | published | catalog@v1.3.0; bin copilot; provider github-copilot; config.local `.github` |
| registry | catalog@v1.3.0 | Additive publish of grok + codex + copilot over v1.2.0 |

## Implementation Guidance

- Prefer installing the agent when practical so version args and path discovery are verified on a real binary; when not installed, open-source clone plus published docs are required, not optional.
- When docs and source disagree, trust source (as `AGENTS.md` states) and note the discrepancy in the research summary shown to the user.
- For provider-agnostic tools, set `agnostic: true` and omit `provider`; do not invent a home provider.
- Keep the outcome log short: result and one-line notes only.
- After each agent, leave the tree in a state the user can publish or discard without leftover half-entries for the next candidate.
- Publish cadence: not after each agent. Default is one registry publish per session (or when the user asks), covering every additive entry already exercised. Session batch published as catalog@v1.3.0 (`grok`, `codex`, `copilot`). Existing installs only re-resolve after the catalog TTL.

## Acceptance Criteria

- Every queue item is either catalogued (entry + matrix when required + validated + exercised), skipped with a recorded reason, or still pending with the queue order intact.
- No catalog entry was added without following the `AGENTS.md` add-agent procedure and a user confirmation of the proposed fields.
- `cue vet ./...` and `cue mod tidy` succeed for the catalog module after each applied entry.
- Each applied entry is visible via `catalog.dir` with `agents get <id>` before it is included in a registry publish.
- When the user requested a registry release, all then-pending exercised entries were included and the outcome log records the publish.
- This project's outcome log reflects the final disposition of each queue item worked during the project.
- Schema-breaking changes were not published as catalog-only; additive publishes did not require a Go release.