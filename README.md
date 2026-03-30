# cv-api

Small Go REST API that serves:
- `GET /github/stats` (GitHub profile/repo stats)
- `GET /lighthouse` (PageSpeed Insights → Lighthouse scores)

## Secrets and token rotation (1Password-first)

This project expects secrets via environment variables. **Do not paste real tokens into `.env`**; store them in 1Password and reference them.

### Required secrets

- **`GITHUB_TOKEN`**: from [GitHub](https://github.com/).
- **`PAGESPEED_API_KEY`**: from [Google Cloud](https://console.cloud.google.com/) for the [PageSpeed Insights API](https://developers.google.com/speed/docs/insights/v5/get-started).

### 1Password — two items

- **`GitHub Token - cv-api`** → field **`token`** → env **`GITHUB_TOKEN`**
- **`Google PageSpeed - cv-api`** → field **`token`** (Google API key) → env **`PAGESPEED_API_KEY`**

If you name the items differently, change the middle segment of each `op://` line to match, or use **Copy secret reference** from each field.

```bash
GITHUB_TOKEN="op://Private/GitHub Token - cv-api/token"
PAGESPEED_API_KEY="op://Private/Google PageSpeed - cv-api/token"
```

(Quotes in `.env` are because item titles contain spaces.)

### PageSpeed Insights API key — step by step (Google Cloud)

1. **Google Cloud project** — [Google Cloud Console](https://console.cloud.google.com/) → select or create a project.
2. **Enable the API** — [PageSpeed Insights API](https://console.cloud.google.com/apis/library/pagespeedonline.googleapis.com) → **Enable**.
3. **Create an API key** — [Credentials](https://console.cloud.google.com/apis/credentials) → **Create credentials** → **API key** → copy the key.
4. **Restrict (recommended)** — edit the key → **API restrictions** → **PageSpeed Insights API** → save.
5. **1Password** — create item **`Google PageSpeed - cv-api`** → field **`token`** → paste the Google key → save.
6. **Check** — `op read "op://Private/Google PageSpeed - cv-api/token" >/dev/null && echo ok`
7. **Run** — `make run` (or `op run --env-file=.env -- go run .`)

### Rotate secrets

- **GitHub** — new token at GitHub → update **`token`** on **`GitHub Token - cv-api`** → revoke the old token.
- **PageSpeed** — new key in [Google Cloud Credentials](https://console.cloud.google.com/apis/credentials) → update **`token`** on **`Google PageSpeed - cv-api`** → delete/restrict the old key.

### Local dev

1. Copy `.env.example` → `.env` and set `GITHUB_USERNAME` / `LIGHTHOUSE_URL` as needed.
2. Run **`make run`** — the Makefile uses **`op run --env-file=.env`** so `op://…` references resolve. Do **not** use `source .env` or plain `go run .`; that leaves secrets as literal `op://` strings and GitHub/PageSpeed calls fail (502).

### If `op read` fails

1. `op item list --vault Private` — item names must match the middle `.env` segment exactly (or use the item UUID in `op://`).
2. **Copy secret reference** per field in 1Password when in doubt.
