# agentdex context

Project-local AI context library for terminal coding agents researched while expanding the agentdex catalog. Mirrors the layout of `~/.agents/context` (indexes, repos.csv, refresh scripts) but only holds agent sources used by this repository.

## Layout

- `index.csv` — inventory of child directories
- `indexes/` — per-source navigation CSVs
- `repos.csv` — clone/update definitions for `scripts/refresh-repos`
- `docs.csv` — optional standalone doc downloads for `scripts/refresh-docs`
- Agent clones (`openai-codex/`, `github-copilot-cli/`, …) — gitignored

## Refresh

```bash
.agents/context/refresh-context
```

Requires git, bash 5+, and the scripts under `scripts/` (executable).

## Agents currently mirrored

| directory | upstream |
|---|---|
| openai-codex | https://github.com/openai/codex |
| github-copilot-cli | https://github.com/github/copilot-cli |

See `index.csv` and `indexes/*.csv` for navigation.
