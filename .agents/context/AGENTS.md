# AGENTS.md

## Repository Overview

Project-local context library under `.agents/context/` for agentdex. Holds shallow clones of terminal AI coding agents used as primary sources when cataloguing outside facts (bin, config paths, skills roots, version flags, providers). Prefer these trees over web queries when researching catalog entries.

## Repository Structure

```txt
.agents/context/
├── index.csv           # Root index - child directories inventory
├── indexes/            # Child indexes (tracked)
│   └── <repo-index>.csv
├── repos.csv           # Clone/update list for refresh-repos
├── docs.csv            # Standalone web docs list (refresh-docs)
├── README.md
├── AGENTS.md
├── refresh-context
├── .gitignore
├── docs/               # Local docs (tracked; optional)
├── cli/                # CLI notes (tracked; optional)
├── scripts/
│   ├── refresh-docs
│   └── refresh-repos
├── <cloned-repo>/      # gitignored clone
└── <cloned-repo>/      # gitignored clone
```

Child clone directories are gitignored. Indexes and scripts are tracked.

## Adding a catalogued agent source

```bash
cd .agents/context
git clone --depth=1 [repository-url] [directory-name]
```

Sparse checkout when only docs matter:

```bash
git clone --filter=blob:none --sparse [repository-url] [directory-name]
cd .agents/context/[directory-name]
git sparse-checkout set [path-to-docs]
```

Then:

1. Add a row to `repos.csv` (`url,directory,sparse_paths,ref`).
2. Create `indexes/{dirname}.csv` (file,description,topics).
3. Add a row to root `index.csv`.

Root index.csv fields: directory, description, topics, source_url, source_type, last_updated.

source_type values: git-sparse-checkout, official-docs, official-tool-repo, local-docs.

Root index lists child directories only, never root-level files.

## Maintenance

```bash
.agents/context/refresh-context
```

Or a single repo:

```bash
cd .agents/context/[repo-name]
git pull --depth=1
```

## Indexing for agentdex catalog work

When indexing agent clones, prefer outside facts useful to catalog entries:

- README and install docs (bin name, install methods)
- Config and skills path docs or source modules that define discovery roots
- Version flag / CLI entrypoints
- Changelog notes that change path conventions

Exclude contributor-only and CI/test trees from child indexes unless they are the only place path constants live (then point at the specific module path).
