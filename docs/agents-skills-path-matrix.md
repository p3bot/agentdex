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
| amp | ~/.agents/skills | ~/.config/amp/skills | ~/.config/agents/skills, ~/.claude/skills | agents |
| augment | ~/.agents/skills | ~/.augment/skills | ~/.claude/skills | agents |
| claude-code | | ~/.claude/skills | | native |
| cline | ~/.agents/skills | ~/.cline/skills | | agents |
| codewhale | ~/.agents/skills | ~/.codewhale/skills | ~/.claude/skills, ~/.deepseek/skills | agents |
| codex | ~/.agents/skills | ~/.codex/skills | | agents |
| copilot | ~/.agents/skills | ~/.copilot/skills | | agents |
| crush | ~/.agents/skills | ~/.config/crush/skills | ~/.config/agents/skills, ~/.claude/skills | agents |
| forge | ~/.agents/skills | ~/.forge/skills | | agents |
| goose | ~/.agents/skills | ~/.config/goose/skills | ~/.claude/skills, ~/.config/agents/skills | agents |
| grok | ~/.agents/skills | ~/.grok/skills | ~/.claude/skills, ~/.cursor/skills | agents |
| hermes | | ~/.hermes/skills | | native |
| kilo | ~/.agents/skills | ~/.config/kilo/skills | ~/.config/kilo/skill, ~/.claude/skills | agents |
| kimi-code | ~/.agents/skills | ~/.kimi-code/skills | | agents |
| kiro | | ~/.kiro/skills | | native |
| opencode | ~/.agents/skills | | ~/.claude/skills | agents |
| open-interpreter | ~/.agents/skills | ~/.openinterpreter/skills | | agents |
| openclaw | ~/.agents/skills | ~/.openclaw/skills | ~/.openclaw/workspace/skills, ~/.openclaw/workspace/.agents/skills | agents |
| openhands | ~/.agents/skills | ~/.openhands/skills | ~/.openhands/microagents | agents |
| pi | ~/.agents/skills | ~/.pi/agent/skills | | agents |
| qwen-code | ~/.agents/skills | ~/.qwen/skills | | agents |

## Local

| agent | agents | native | alternatives | primary |
|---|---|---|---|---|
| agy | .agents/skills | | .claude/skills, .opencode/skills | agents |
| amp | .agents/skills | | .claude/skills | agents |
| augment | .agents/skills | .augment/skills | .claude/skills | agents |
| claude-code | | .claude/skills | | native |
| cline | .agents/skills | .cline/skills | .clinerules/skills | agents |
| codewhale | .agents/skills | .codewhale/skills | skills, .opencode/skills, .claude/skills, .cursor/skills | agents |
| codex | .agents/skills | .codex/skills | | agents |
| copilot | .agents/skills | .github/skills | .claude/skills | agents |
| crush | .agents/skills | .crush/skills | .claude/skills, .cursor/skills | agents |
| forge | | .forge/skills | | native |
| goose | .agents/skills | .goose/skills | .claude/skills | agents |
| grok | .agents/skills | .grok/skills | .claude/skills, .cursor/skills | agents |
| kilo | .agents/skills | .kilo/skills | .kilo/skill, .claude/skills | agents |
| kimi-code | .agents/skills | .kimi-code/skills | | agents |
| kiro | | .kiro/skills | | native |
| opencode | .agents/skills | .opencode/skills | .claude/skills | agents |
| open-interpreter | .agents/skills | .openinterpreter/skills | | agents |
| openhands | .agents/skills | .openhands/skills | .openhands/microagents | agents |
| pi | .agents/skills | .pi/skills | | agents |
| qwen-code | .agents/skills | .qwen/skills | | agents |

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
| amp | global | ~/.claude/plugins/cache | n/a (dynamic Claude plugin cache) |
| amp | any | amp.skills.path | n/a (user-configured extra roots) |
| kilo | local | .opencode/skills | No (no longer loaded; use .kilo) |
| kilo | local | .kilocode/skills | legacy (prefer .kilo) |
| open-interpreter | any | ~/.config/open-interpreter | No (legacy Python open-interpreter; not the Rust terminal product) |
| forge | global | ~/forge | legacy (used as base_path when directory exists; default is ~/.forge) |
| forge | local | .agents/skills | No (global ~/.agents/skills only; local skills are .forge/skills) |
| forge | any | forge.yaml | n/a (project config file, not a config directory) |
| hermes | global | ~/.agents/skills | No (skills live under ~/.hermes/skills; .agents only as hub import sources) |
| hermes | local | any project skills root | No (skills are HERMES_HOME-scoped only) |
| hermes | any | skills.external_dirs | n/a (user-configured extra roots in config.yaml) |
| openclaw | global | ~/.clawdbot | legacy (used as state dir when ~/.openclaw is missing) |
| openclaw | local | project cwd skills | No (skills load from configured workspace, default ~/.openclaw/workspace, not cwd auto-discovery) |
| openclaw | any | skills.load.extraDirs | n/a (user-configured extra roots) |
| openclaw | any | $CODEX_HOME/skills | No (Codex native skills not loaded; migrate via openclaw migrate codex) |

## Authoring notes

- Agent id is the catalog map key (kebab-case).
- alternatives lists are priority order (leftmost is primary fallback).
- When adding or changing skills on a catalog entry, update the Global/Local
  rows for that agent and any matching Unsupported rows.
