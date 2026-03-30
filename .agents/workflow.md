# Workflow (cv-api)

## Code references
When citing existing files in chat, use Cursor-style ranges so links are jumpable:

`@path/to/file (startLine-endLine)` — e.g. `@.env.example (5-6)` for the comment and `LIGHTHOUSE_URL`.

## Version control
- Use **`jj`** only — never `git` CLI.
- All automated `jj` commands include **`--no-pager`**.
- Prefer **JJ MCP** (Jujutsu MCP in Cursor) for status, log, diff, bookmarks, and `git push` flows when it is configured; otherwise use **`jj`** from the terminal per `/.agents/skills/jj/SKILL.md` and [jj docs](https://docs.jj-vcs.dev/).

### Sync with GitHub before new work (pull + base = `main`)
Default branch is **`main`** (there is no `master` in these repos).

1. `jj git fetch --remote origin`
2. `jj new main` — creates a **new empty change** on top of the current **`main`** bookmark (after fetch, `main` matches `main@origin` when tracking is set). Your working copy tree matches the latest merged `main`; commit new work here.

To **rebase an existing feature bookmark** onto updated `main` instead:  
`jj rebase -d main@origin -r <bookmark-name>` (adjust revset if needed).

### Pull requests — do not land features by pushing `main`
1. Start work on top of **`main`**: `jj new main` (creates a new change; working copy may be empty until you edit — that is normal).
2. Make commits on your change as usual: `jj commit -m '…'` (with paths if splitting).
3. **Push a feature branch for GitHub PR**, not `main`:
   - **Option A — named bookmark:**  
     `jj bookmark create <feature-name> -r @` → `jj bookmark track <feature-name>` (if needed) → `jj git push --remote origin --bookmark <feature-name> --allow-new` (first push of a new bookmark).
   - **Option B — auto bookmark name:**  
     `jj git push --change @-` (or `--change @`) so jj creates/pushes a bookmark for that revision ([docs](https://docs.jj-vcs.dev/latest/github)).
4. Open a **Pull Request** in the **GitHub web UI** — not `gh` CLI. After `jj git push`, GitHub prints a link such as `https://github.com/<owner>/<repo>/pull/new/<bookmark>`. Use that (or **Compare & pull request** on the repo). Merge on GitHub when ready. Do not land feature work by force-pushing local `main`.

### Empty working copy after `jj new`
`jj new` creates an **empty** child change on purpose. Bookmarks are **not** created automatically unless you set one or use `jj git push --change …`. If you bookmark an **empty** revision, jj may **warn** — usually you commit first, then `jj bookmark move <name> --to @` or push with `--change`.

### Deploy
- **`flyctl deploy`** (see `Makefile` `deploy` / `/.agents/skills/flyio/SKILL.md`). Production secrets live in Fly, not in `op run` local `.env`.

## API testing
- Use **REST Client** (`api.http`) — never curl.
