# Context maintenance

Maintain indexes and clones under `.agents/context/` for agentdex catalog research.

## Rules

- Root `index.csv` lists child directories only
- Each clone has `indexes/{dirname}.csv`
- Each clone is listed in `repos.csv` so refresh-repos can update it
- Clones are gitignored; indexes and scripts are tracked
- Index for outside-facts research (paths, bin, skills, providers), not contributor noise

## When adding an agent clone

1. Shallow clone into `.agents/context/{dirname}`
2. Append `repos.csv`
3. Write `indexes/{dirname}.csv`
4. Append root `index.csv` with last_updated
5. Prefer primary sources already cloned here when proposing catalog entries
