# nyt — full command reference

Every command, argument, flag, and output shape. Read the section you need; the
SKILL.md covers the workflow and when to pick each command.

All commands accept the **global flags**: `--api-key`, `--json`, `-v/--verbose`,
`--timeout` (default 30s), `--throttle`, `--retries` (default 3).

## Contents
- [topstories](#topstories) — current section fronts
- [articlesearch](#articlesearch) — keyword search back to 1851
- [archive](#archive) — every article in a month
- [mostpopular](#mostpopular) — most viewed/emailed/shared
- [books](#books) — best-seller lists
- [rss](#rss) — public real-time feeds (no key)
- [read](#read) — full article text (needs cookie)
- [config](#config) — manage the saved API key
- [version](#version)

---

## topstories
Articles currently on a NYTimes.com section front (Top Stories API).

```
nyt topstories [section]
```
- `section` defaults to `home`. Examples: `technology`, `world`, `business`, `us`.
- `--sections` — list valid section names and exit.

```sh
nyt topstories
nyt topstories technology
nyt topstories world --json
nyt topstories --sections
```
**Output:** table of `# / SECTION / TITLE / BYLINE`. JSON: NYT envelope with
`results[]`; the article link is `results[].url`.

---

## articlesearch
Search NYT articles by keyword and Lucene filter (Article Search API). Coverage
goes back to 1851.

```
nyt articlesearch [query]
```
The positional `query` sets `q`. A query-less search is allowed if `--fq` is
given. Results are paginated **10 per page**.

| Flag | Meaning |
|---|---|
| `--begin <date>` | earliest pub date (`YYYY-MM-DD` or `YYYYMMDD`) |
| `--end <date>` | latest pub date |
| `--sort <order>` | `newest` \| `oldest` \| `relevance` (default `relevance`) |
| `--page <n>` | result page, `0..100` |
| `--fq <lucene>` | filter query, e.g. `section_name:("Sports")` |
| `--fl <fields>` | comma-separated fields to return |
| `--facet <field>` | facet field to compute (repeatable) |
| `--facet-filter` | apply `fq` when computing facet counts |
| `--hl` | highlight search terms in snippet/headline |

```sh
nyt articlesearch "climate change"
nyt articlesearch "election" --begin 2024-01-01 --end 2024-11-05 --sort newest
nyt articlesearch --fq 'section_name:("Sports") AND type_of_material:("News")'
nyt articlesearch "ai" --fl web_url,headline,pub_date --json
```
**Output:** table of results. JSON: `response.docs[]`; the article link is
`docs[].web_url`, headline at `docs[].headline.main`, summary at `docs[].abstract`.

---

## archive
Every article NYT published in a given month (Archive API).

```
nyt archive <year> <month>
```
- `year` 1851–2100; `month` 1–12 (leading zero ok).
- Responses are **large (~20MB/month)** — raise `--timeout` for old/big months.

| Flag | Meaning |
|---|---|
| `--limit <n>` | max table rows (`0` or negative = all; default 20) |
| `--section <name>` | filter rows by `section_name` (case-insensitive substring) |

```sh
nyt archive 2024 3
nyt archive 2020 1 --timeout 120s
nyt archive 2024 3 --section "Sports" --limit 50
nyt archive 2024 3 --limit 0 --json
```
**Output:** table. JSON: `response.docs[]`, link at `docs[].web_url`.

---

## mostpopular
Most viewed, emailed, or shared articles (Most Popular API).

```
nyt mostpopular <metric> [period]
```
- `metric` (required): `viewed` | `emailed` | `shared`.
- `period` (optional): `1` | `7` | `30` days (default `7`).
- `--share-type <platform>` — only valid with `shared`; `facebook` is reliable.

```sh
nyt mostpopular viewed
nyt mostpopular emailed 1
nyt mostpopular shared 7 --share-type facebook
nyt mostpopular viewed 30 --json
```
**Output:** table. JSON: `results[]`, link at `results[].url`.

---

## books
NYT best-seller lists (Books API). All data from `/svc/books/v3`.

```
nyt books lists                    # available list names (+ encoded names)
nyt books list <list-name>         # one current (or dated) list
nyt books overview                 # top 5 of every list for a date
```

Start with `nyt books lists` to discover the **encoded** names that `books list`
expects (e.g. `hardcover-fiction`, `combined-print-and-e-book-fiction`).

`books list <list-name>` flags:
| Flag | Meaning |
|---|---|
| `--date <YYYY-MM-DD>` | list as published nearest this date (default: current) |
| `--offset <n>` | results offset, **multiple of 20** |
| `--sort-by <field>` | e.g. `rank`, `title`, `author` |
| `--sort-order <ASC\|DESC>` | sort direction |

`books overview` accepts `--date <YYYY-MM-DD>`.

```sh
nyt books lists
nyt books list hardcover-fiction
nyt books list hardcover-fiction --date 2024-01-07
nyt books overview --date 2024-01-07
```
**Note on JSON shape:** for `books list`, `results` is an *object* containing a
`books[]` array (not an array of entries directly).

---

## rss
Latest articles by section from NYT's **public RSS feeds**. Free, **no API key**.

```
nyt rss [section]
```
- `section` defaults to `HomePage`. Any feed name NYT publishes is accepted.
- `--limit <n>` — max items (default 20).
- `--sections` — list known feed names and exit.

```sh
nyt rss
nyt rss technology
nyt rss world --limit 10
nyt rss --sections
```
**Output:** table of `# / TITLE / BYLINE / PUBLISHED`. Because NYT serves RSS as
XML, `--json` prints a **normalized** JSON view — not a raw upstream
passthrough. Shape: `{section, link, updated, num_items, items[]}` where each
item is `{title, url, byline, published, abstract, categories}`. Use `--json` to
get item URLs (the per-item link is `items[].url`).

---

## read
Fetch and extract the **full readable text** of a nytimes.com article.

```
nyt read <url>
```
The NYT APIs only return metadata, so the body is pulled directly from
nytimes.com. This needs the user's browser **cookie** (subscription + DataDome),
resolved: `--cookie` flag > `$NYT_COOKIE` > `.env` > `~/.config/nyt/config.json`.
**No API key is used.** Pass an absolute `http(s)` nytimes.com URL.

- `--cookie <value>` — override the cookie (otherwise from env/config).

```sh
nyt read "https://www.nytimes.com/2026/06/22/world/example.html"
nyt read "https://www.nytimes.com/2026/06/22/world/example.html" --json
```
**Output (table mode):** title, byline, then the clean article text (image
captions, ads, and recirculation blocks stripped). **JSON mode:** a lean object
`{url, title, author, published, excerpt, text}` — ideal for summarizing or
quoting. The raw HTML is deliberately omitted to keep it small.

**Failure mode:** a 403 means the cookie expired or is invalid → "refresh
NYT_COOKIE from DevTools → Application → Cookies". The cookie must include the
`NYT-S` and `datadome` values. Relay this to the user; there's no workaround.

---

## config
Manage the saved API key and configuration.

```
nyt config show       # where the key resolves from (value redacted)
nyt config set-key <KEY>   # save key to ~/.config/nyt/config.json
nyt config path       # print the config file path
```
`config show` is the quickest way to confirm auth is set up before running a
key-requiring command.

---

## version
```
nyt version
```
Prints the build version and platform (e.g. `nyt dev (darwin/arm64, go1.26.3)`).
