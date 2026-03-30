---
name: jj
description: Look up jj (Jujutsu) VCS docs via context7 before running version control commands
---

Use context7 with library ID `/martinvonz/jj` for jj docs.

Rules:
- Always use `jj` — never `git`
- All jj commands must include `--no-pager`
- Never commit without asking the user first
- Prefer **JJ MCP** when configured in Cursor; otherwise run `jj` in the shell
- Do not push feature work directly to `main` — use a feature bookmark / `jj git push --change` and GitHub PRs (see `/.agents/workflow.md`)
