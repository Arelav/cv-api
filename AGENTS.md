# CV API

Go REST API. Serves GitHub activity stats and Lighthouse scores to the cv frontend.

## Version Control
- Use `jj` — do not use `git` commands
- All automated `jj` commands must include `--no-pager`

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
