# nyt

A command-line wrapper for the [New York Times developer APIs](https://developer.nytimes.com/apis),
written in Go. It covers Top Stories, Article Search, the Archive, Most Popular,
the Books best-seller lists, and the public RSS feeds — each as a subcommand
with human-readable tables by default and raw JSON on demand. It can also fetch
the full readable text of any nytimes.com article (`nyt read`).

```
$ nyt topstories technology
Top Stories — Technology (updated 2026-06-18T12:13:00-04:00)
────────────────────────────────────────────────────────────
#   SECTION     TITLE                                                       BYLINE
1   technology  Tech Workers Maxed Out Their A.I. Use. Now They're Tryin…   Eli Tan
2   technology  New Super PAC Aims to Rally Tech Workers to Help Limit A.I.  Mike Isaac and Theodo…
...
```

## Install

Requires Go 1.22+.

```sh
git clone <this repo> && cd nyt_cli
go build -o nyt .        # produces ./nyt
# or install onto your PATH (into $(go env GOPATH)/bin):
go install .
```

## Authentication

Every request needs your NYT app **Key** (the "Key" shown for an app at
<https://developer.nytimes.com/my-apps>). The Secret is not used by these REST
APIs. The key is resolved in this priority order:

1. `--api-key` flag
2. `$NYT_API_KEY` or `$NYT_KEY` environment variable
3. a `.env` file in the current directory or any parent (auto-loaded)
4. `~/.config/nyt/config.json` (managed with `nyt config set-key`)

```sh
# Option A: environment / .env
echo 'NYT_KEY=your-key-here' >> .env

# Option B: saved config file
nyt config set-key your-key-here
nyt config show          # prints the source, value redacted
```

> Each API must be individually enabled for your app in the developer portal.
> If a command returns `401 Invalid ApiKey for given resource` or a `404`, enable
> that API for your app at <https://developer.nytimes.com/my-apps>.

## Global flags

| Flag | Default | Description |
|------|---------|-------------|
| `--api-key` | — | API key (overrides env/config) |
| `--json` | `false` | print the raw NYT JSON response instead of a table |
| `-v, --verbose` | `false` | log requests to stderr (key redacted) |
| `--timeout` | `30s` | per-request timeout (raise it for `archive`) |
| `--throttle` | `0` | minimum delay between requests, e.g. `--throttle 6s` |
| `--retries` | `3` | retries on 429 / 5xx / transport errors (honors `Retry-After`) |

## Commands

### Top Stories
```sh
nyt topstories                 # defaults to the "home" front
nyt topstories technology
nyt topstories --sections      # list valid sections
```

### Article Search
```sh
nyt articlesearch "climate change"
nyt articlesearch "climate" --begin 2024-01-01 --end 2024-12-31 --sort newest
nyt articlesearch --fq 'section_name:("Sports") AND type_of_material:("News")'
nyt articlesearch "ai" --page 2 --facet section_name --json
```
Dates accept `YYYY-MM-DD` or `YYYYMMDD`. Results are 10 per page (pages 0–100).

### Archive
```sh
nyt archive 2024 3                       # every article from March 2024
nyt archive 2020 1 --timeout 120s        # big months can be ~20MB
nyt archive 2024 3 --section Sports --limit 50
nyt archive 2024 3 --limit 0 --json      # all rows / raw JSON
```

### Most Popular
```sh
nyt mostpopular viewed 7                  # most viewed, last 7 days
nyt mostpopular emailed 1
nyt mostpopular shared 30 --share-type facebook
```
Metric is `viewed|emailed|shared`; period is `1|7|30` (default 7).

### Books
```sh
nyt books lists                          # available best-seller lists (+ encoded names)
nyt books list hardcover-fiction         # a current list
nyt books list hardcover-fiction --date 2024-01-07
nyt books overview                       # top 5 of every list
```

### RSS (real-time feed)
```sh
nyt rss                                  # the HomePage feed
nyt rss technology
nyt rss world --limit 10
nyt rss business --json
nyt rss --sections                       # known feed names
```
Backed by NYT's free public RSS feeds (no API key needed). The default section
is `HomePage`; any feed name NYT publishes is accepted. Because NYT serves RSS
as XML, `--json` here prints a **normalized** representation of the feed rather
than a raw upstream passthrough.

### Read (full article text)
```sh
nyt read "https://www.nytimes.com/2026/06/22/world/example.html"
nyt read "<url>" --json
```
Fetches a nytimes.com article URL and prints its readable full text (the NYT
APIs only return metadata, so the body is pulled directly from nytimes.com).
This needs your **browser cookie**, not an API key, supplied (in priority order)
via `--cookie`, `$NYT_COOKIE`, a `.env` file, or `~/.config/nyt/config.json`.
Copy the cookie from DevTools → Application → Cookies (it must include the
`NYT-S` and `datadome` values). Cookies rotate: when yours expires the fetch is
blocked and you'll be told to refresh it. `--json` prints a lean shape (title,
author, published, excerpt, text) with the raw HTML omitted.

### Config & version
```sh
nyt config set-key <KEY>     nyt config path     nyt config show
nyt version
```

## Notes

- **Retired NYT APIs are not included.** NYT has decommissioned several APIs —
  Movie Reviews, Times Newswire, the Semantic API, and TimesTags — and its
  developer portal no longer lets you enable them (requests return 401/404). The
  CLI exposes only endpoints that are live. For movie coverage use
  `nyt articlesearch` or `nyt topstories movies`; the `rss` command replaces the
  retired Times Newswire real-time feed.
- Pass `--json` to any command to get the raw upstream response — useful for
  scripting with `jq`. The one exception is `rss`: NYT serves it as XML, so
  `--json` prints a normalized JSON view of the feed instead.
- Rate limits are per app (commonly ~5 req/s and a daily cap). The client retries
  `429`/`5xx` with exponential backoff and honors `Retry-After`; add `--throttle`
  for tight loops.

## Project layout

```
main.go                 entry point
cmd/                    one file per command (self-registering via init())
internal/client/        HTTP client: auth, retries, rate-limit handling, errors
internal/config/        API-key resolution (flag/env/.env/config) + dotenv loader
internal/output/        JSON and table rendering helpers
```
```sh
go test ./...           # (build/vet)
go build ./...
```
