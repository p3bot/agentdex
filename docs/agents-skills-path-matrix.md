# Agent skills path matrix

Catalog roles per scope: agents (shared `.agents`), native (product tree),
alternatives (compat roots, priority order). Primary is derived at runtime:
agents, else native, else alternatives[0]. Not stored in the catalog.

Empty cells mean the path is not used for that role. Explicit No and n/a from
research live in the Unsupported section so catalog-shaped Yes data stays
comparable.

## Global

| agent | agents | native | alternatives | primary |
|---|---|---|---|---|
| agy | | ~/.gemini/antigravity-cli/skills | | native |
| augment | ~/.agents/skills | ~/.augment/skills | ~/.claude/skills | agents |
| claude-code | | ~/.claude/skills | | native |
| cline | ~/.agents/skills | ~/.cline/skills | | agents |
| codewhale | ~/.agents/skills | ~/.codewhale/skills | ~/.claude/skills, ~/.deepseek/skills | agents |
| codex | ~/.agents/skills | ~/.codex/skills | | agents |
| copilot | ~/.agents/skills | ~/.copilot/skills | | agents |
| goose | ~/.agents/skills | ~/.config/goose/skills | ~/.claude/skills, ~/.config/agents/skills | agents |
| grok | ~/.agents/skills | ~/.grok/skills | ~/.claude/skills, ~/.cursor/skills | agents |
| kiro | | ~/.kiro/skills | | native |
| opencode | ~/.agents/skills | | ~/.claude/skills | agents |

## Local

| agent | agents | native | alternatives | primary |
|---|---|---|---|---|
| agy | .agents/skills | | .claude/skills, .opencode/skills | agents |
| augment | .agents/skills | .augment/skills | .claude/skills | agents |
| claude-code | | .claude/skills | | native |
| cline | .agents/skills | .cline/skills | .clinerules/skills | agents |
| codewhale | .agents/skills | .codewhale/skills | skills, .opencode/skills, .claude/skills, .cursor/skills | agents |
| codex | .agents/skills | .codex/skills | | agents |
| copilot | .agents/skills | .github/skills | .claude/skills | agents |
| goose | .agents/skills | .goose/skills | .claude/skills | agents |
| grok | .agents/skills | .grok/skills | .claude/skills, .cursor/skills | agents |
| kiro | | .kiro/skills | | native |
| opencode | .agents/skills | .opencode/skills | .claude/skills | agents |

## Unsupported

Researched paths that are not supported (No) or not applicable (n/a). Not
stored in the catalog.

| agent | scope | path | support |
|---|---|---|---|
| agy | global | ~/.agents/skills | No |
| agy | global | ~/.claude/skills | No |
| claude-code | global | ~/.agents/skills | No |
| claude-code | local | .agents/skills | No |
| copilot | global | ~/.claude/skills | No |
| copilot | local | .opencode/skills | No |
| goose | global | ~/.agents/plugins/*/skills | n/a (dynamic plugin globs; not a static catalog path) |
| kiro | global | ~/.agents/skills | No |
| kiro | local | .agents/skills | No |
| cline | local | .claude/skills | No (docs list it; source search paths omit it) |
| opencode | global | ~/.copilot/skills | n/a |

## Authoring notes

- Agent id is the catalog map key (kebab-case).
- alternatives lists are priority order (leftmost is primary fallback).
- When adding or changing skills on a catalog entry, update the Global/Local
  rows for that agent and any matching Unsupported rows.
