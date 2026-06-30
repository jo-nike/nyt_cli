---
name: nyt-cli
description: >-
  Fetch New York Times content by running the bundled `nyt` command-line tool:
  current top stories and section fronts, full-text article search back to 1851,
  a whole month's archive, most-viewed/emailed/shared articles, Books
  best-seller lists, real-time RSS feeds, and the complete readable text of any
  nytimes.com article URL. Use this whenever the user wants New York Times news,
  NYT headlines, NYT article search, NYT best-sellers, what's trending/popular on
  NYT, recent NYT coverage of a topic, or the full text of a specific
  nytimes.com link — even if they never say "nyt" or "CLI". Prefer this tool over
  scraping nytimes.com or guessing URLs; it handles auth, rate limits, and
  article-text extraction for you.
---

# nyt — New York Times from the command line

`nyt` is a CLI that wraps the New York Times developer APIs plus its public RSS
feeds and on-page article text. Reach for it any time the user wants something
from the NYT: a topic's recent coverage, today's headlines, a search, the
best-seller lists, what's popular, or the full body of a specific article.

The compiled binary is **bundled with this skill** — you don't need the source
repo. Everything below is about choosing the right command and avoiding the few
sharp edges.

## Running the bundled binary

The binary lives at `bin/nyt` inside this skill's directory (the absolute base
directory is shown to you when the skill loads). Shell environment does **not**
persist between separate command calls, so use the full path on every
invocation rather than relying on an exported variable or alias. For example:

```sh
/abs/path/to/nyt-cli/bin/nyt topstories technology
```

If a `nyt` is already on the user's `PATH`, that's the same tool and you can call
it bare (`nyt topstories ...`) — but the bundled binary is the reliable default.

Two things that can stop the bundled binary cold:
- **Wrong platform.** It's built for macOS arm64. On any other OS/arch it won't
  execute — rebuild with `go install gitea.jonn.me/jons-org/nyt_cli@latest`
  (Go 1.22+) or `go build` from the source repo, and call that binary instead.
  You can also grab a prebuilt binary for your OS/arch from the repo's releases.
- **macOS quarantine** (only if it was downloaded/repackaged, not built locally):
  if you see "cannot be opened", clear it with
  `xattr -d com.apple.quarantine /abs/path/to/bin/nyt`.

## Authentication — two separate models

Most commands hit the NYT APIs and need the app **API key**. The lone exception
on the read side is `read`, which fetches the article off nytimes.com and needs
the user's **browser cookie** instead. `rss` needs no auth at all.

**API key** (for topstories, articlesearch, archive, mostpopular, books) resolves
in this order — first hit wins:
1. `--api-key` flag
2. `$NYT_API_KEY` or `$NYT_KEY`
3. a `.env` file in the working dir or any parent (auto-loaded)
4. `~/.config/nyt/config.json` (managed via `nyt config set-key <KEY>`)

Check what's configured with `nyt config show` (it prints the source, key
redacted). If nothing resolves, the command fails with a clear auth error — tell
the user to set `NYT_KEY` or run `nyt config set-key`, rather than guessing a key.

**Cookie** (for `read` only) resolves `--cookie` > `$NYT_COOKIE` > `.env` >
config. It's the user's logged-in nytimes.com cookie (must contain the `NYT-S`
subscription value and the `datadome` value, copied from DevTools → Application →
Cookies). Cookies rotate: when one expires, `read` returns a clear "cookie
expired or invalid" error — relay that and ask the user to refresh it. You can't
fabricate or work around this.

## Choosing the command

Match the user's intent to a command. When unsure of valid section/list names,
**discover them** rather than guessing (see the gotchas).

| The user wants… | Command |
|---|---|
| Today's headlines / what's on a section front now | `topstories [section]` |
| A lightweight real-time feed of latest articles (no API key) | `rss [section]` |
| To search articles by keyword/topic, optionally by date or filter | `articlesearch "<query>"` |
| Everything NYT published in a specific month | `archive <year> <month>` |
| The most viewed / emailed / shared articles lately | `mostpopular <viewed\|emailed\|shared> [1\|7\|30]` |
| Best-seller book lists | `books lists` · `books list <name>` · `books overview` |
| The **full text** of a specific nytimes.com article | `read <url>` |

`topstories` and `rss` overlap for "what's the latest in section X". Prefer
`rss` when the user just wants a quick current feed and you want to avoid using
an API-key quota (it's keyless); prefer `topstories` for the curated section
front and richer fields.

Full flags, arguments, and output shapes for every command are in
[references/commands.md](references/commands.md) — read it when you need a flag
you don't remember (date ranges, Lucene `--fq` filters, sort/offset, etc.).

## The key workflow: metadata → full text

This is the most important thing to internalize. **The NYT APIs and RSS feeds
return only *metadata*** — headline, byline, abstract, section, and a URL — never
the article body. So any task that needs the actual article ("summarize this
NYT piece", "what does the article say about X", "read me the top tech story")
is two steps:

1. **Find the article and get its URL.** Run whichever discovery command fits
   (`articlesearch`, `topstories`, `rss`, `mostpopular`) with `--json`, then pipe
   it through the bundled helper to get a ranked, scannable list — one
   `index <TAB> date <TAB> title <TAB> url` row per article. Each command buries
   the URL under a different field; the helper normalizes that so you don't have
   to parse JSON by hand:

   ```sh
   nyt articlesearch "ai in the workplace" --sort newest --json \
     | python3 <skill-dir>/scripts/links.py
   ```

   Scan the dates and titles, pick the row you want, and take its URL. (If you
   ever need the raw field names, they're in references/commands.md; the helper
   just unifies `response.docs[].web_url`, `results[].url`, and `items[].url`.)
2. **Fetch the body** with `nyt read "<url>"` (add `--json` for a structured
   `{title, author, published, text}` object that's easy to quote from).

Do **not** try to `curl` or scrape nytimes.com yourself, or guess an article URL
from a headline — `read` exists precisely because the page is behind DataDome
and a subscription; it handles the cookie, the bot defenses, and strips the page
down to clean article text. Let it do that job.

Example — "summarize today's top technology story":
```sh
nyt topstories technology --json | python3 <skill-dir>/scripts/links.py   # pick a row
nyt read "<that url>" --json                                              # full text → summarize
```

## Tables vs. --json

By default every command prints a compact human-readable **table** — good for
showing the user a list. Switch to `--json` whenever you need to *act on* the
data rather than just display it:
- to extract a URL to feed into `read` (the table truncates and omits links),
- to get fields the table leaves out (abstract, web_url, categories, full byline),
- to filter/process results programmatically (pipe to `jq`).

`--json` is a raw passthrough of the upstream NYT JSON for every command **except
`rss`**, where NYT serves XML, so `--json` there is a *normalized* JSON view of
the feed.

## Gotchas worth knowing

- **Discover names, don't guess.** Valid section/list names come from the tool:
  `nyt topstories --sections`, `nyt rss --sections`, `nyt books lists` (the last
  prints the *encoded* list names that `books list` expects, e.g.
  `hardcover-fiction`).
- **Archive is big.** A single month can be ~20MB and blow the 30s default
  timeout — add `--timeout 120s`, and trim the table with `--limit` / `--section`.
- **Rate limits.** Quotas are per app (~5 req/s plus a daily cap). The client
  already retries 429/5xx with backoff and honors `Retry-After`; in a tight loop
  add `--throttle 6s` so you don't trip the limit.
- **Some NYT APIs are retired and intentionally absent** — Movie Reviews, Times
  Newswire, the Semantic API, TimesTags. Don't try to call them. For movie
  coverage use `articlesearch` or `topstories movies`; `rss` replaces the old
  Times Newswire real-time feed.
- **Argument constraints:** `mostpopular` period is only `1|7|30`;
  `articlesearch` pages are `0..100` (10 results/page); `books list --offset`
  must be a multiple of 20.

## Global flags (work on any command)

`--json` (raw/normalized JSON) · `-v/--verbose` (log requests to stderr, key
redacted) · `--timeout` (default 30s) · `--throttle` (min delay between requests)
· `--retries` (default 3) · `--api-key` (override resolution).
