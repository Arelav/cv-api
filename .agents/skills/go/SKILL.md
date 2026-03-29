---
name: go
description: Look up Go standard library and language docs via context7 before writing Go code
---

Use context7 with library ID `/golang/go` for Go standard library docs.

Stack used in this project:
- `net/http` — stdlib HTTP server, no heavy frameworks
- `encoding/json` — stdlib JSON
- Generics (`cache[T any]`) for typed in-memory caching

Rules:
- Standard library first — avoid adding external dependencies
- Use `go vet ./...` to check for compile errors — not `go build`
- Tests live alongside code (`*_test.go`)
