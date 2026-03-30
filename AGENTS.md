# CV API

Go REST API. Serves GitHub activity stats and Lighthouse scores to the cv frontend.

## Code references (agents)
Use Cursor-style file ranges when pointing at existing code: `@path/to/file (startLine-endLine)` (e.g. `@.env.example (5-6)`).

## Version Control
- Use `jj` — do not use `git` commands
- All automated `jj` commands must include `--no-pager`
- **Do not push feature work straight to `main`.** Use a feature bookmark (or `jj git push --change …`) and open a **GitHub PR** — see `.agents/workflow.md`.
- Prefer **JJ MCP** for jj operations when it is enabled in Cursor; otherwise use the `jj` CLI.

@.agents/workflow.md

## GitHub
- **CI:** `.github/workflows/ci.yml` runs on push/PR to `main` — `go test ./...` (see `go.mod` for the Go version Actions uses).
- **CodeRabbit:** install the [GitHub App](https://github.com/apps/coderabbitai) on this repository once. Configure behavior in `.coderabbit.yaml` ([docs](https://docs.coderabbit.ai/configure-coderabbit)).

## Docs & Research
- Use **context7 MCP** for library docs — not web search first, not guessing from training data
- Use **WebSearch MCP** for troubleshooting when context7 doesn't have the answer
- Skills: `/go`, `/flyio`, `/jj`, `/agent-setup`

## Rules
- Standard library first — avoid heavy frameworks
- Never ignore explicit instructions from the user
- Use REST Client (`api.http`) for API testing — never curl

## Caching
- GitHub API responses: in-memory, TTL configurable
- PageSpeed Insights responses: 24h TTL

## Endpoints (planned)
- GET /health
- GET /github/stats
- GET /lighthouse
