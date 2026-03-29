---
name: agent-setup
description: How to set up and configure AI agents correctly for this project
---

# Agent Setup for This Project

## Before writing any code

1. Read `AGENTS.md` — it has rules that override default behavior
2. Use context7 to fetch docs for the relevant library — never assume API from training data
3. Invoke the relevant skill: `/tailwind`, `/nextjs`, `/biome`, `/typescript`, `/tanstack-query`, `/fontawesome`, `/jj`

## MCP servers configured

- **context7** — up-to-date library docs. Always prefer this over training data or node_modules
- **WebSearch** — for troubleshooting, issues not in context7

## How to use context7

Call `resolve_library_id` with the library name, then `get-library-docs` with the returned ID and a focused topic.

## Commits

- Never commit without asking the user first
- Ask at natural checkpoints, not after every file change

## Do not

- Spawn subagents to search — do it yourself with context7 or WebSearch
- Read docs from `node_modules/`
- Use `git` — use `jj --no-pager`
- Rename props to match component names — keep data field names as-is
- Add ESLint, Prettier, CSS modules, inline styles
- Commit without asking
