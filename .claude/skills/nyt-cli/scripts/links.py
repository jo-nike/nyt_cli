#!/usr/bin/env python3
"""Turn `nyt <discovery-command> --json` into a ranked, pick-and-read list.

The NYT discovery commands each bury the article URL under a different field,
which makes "search, then read the full text" a fiddly two-step where you have
to remember the shape. Pipe the `--json` output through this helper instead and
get one tab-separated row per article — `index <TAB> date <TAB> title <TAB> url`
— across every shape:

  - articlesearch / archive : response.docs[].web_url   / headline.main / pub_date
  - topstories / mostpopular: results[].url             / title         / published_date
  - rss (--json)            : items[].url               / title         / published

Usage:
  nyt articlesearch "ai in the workplace" --sort newest --json | python3 links.py
  nyt topstories technology --json | python3 links.py | head
  nyt rss world --json | python3 links.py

Then pick the URL you want and read the body:
  nyt read "<url>" --json

Exit status is non-zero (with a hint on stderr) if stdin isn't JSON or no
articles are found, so failures are obvious rather than silent.
"""
import sys
import json


def field(d, *paths):
    """Return the first non-empty string at any of the dotted paths."""
    for path in paths:
        v = d
        for part in path.split("."):
            if isinstance(v, dict) and part in v:
                v = v[part]
            else:
                v = None
                break
        if isinstance(v, str) and v.strip():
            return v.strip()
    return ""


def find_items(data):
    """Locate the list of article objects regardless of command shape."""
    if isinstance(data, list):
        return data
    if isinstance(data, dict):
        if isinstance(data.get("results"), list):
            return data["results"]
        resp = data.get("response")
        if isinstance(resp, dict) and isinstance(resp.get("docs"), list):
            return resp["docs"]
        if isinstance(data.get("items"), list):
            return data["items"]
    return None


def short_date(s):
    """Trim ISO timestamps (2026-06-01T..) to the date; leave others as-is."""
    if len(s) >= 10 and s[:4].isdigit() and s[4] == "-":
        return s[:10]
    return s


def main():
    raw = sys.stdin.read()
    try:
        data = json.loads(raw)
    except json.JSONDecodeError:
        sys.stderr.write("links.py: stdin is not JSON — did you forget --json? "
                         "(note: rss --json is normalized and also works)\n")
        return 2

    items = find_items(data)
    if items is None:
        sys.stderr.write("links.py: couldn't find an article list in this JSON "
                         "(expected results[], response.docs[], or items[])\n")
        return 2

    rows = []
    for it in items:
        if not isinstance(it, dict):
            continue
        url = field(it, "url", "web_url")
        if not url:
            continue
        title = field(it, "title", "headline.main", "headline")
        date = short_date(field(it, "pub_date", "published_date", "published",
                                "created_date", "updated_date", "updated",
                                "first_published_date"))
        rows.append((date, title, url))

    if not rows:
        sys.stderr.write("links.py: no articles with URLs found\n")
        return 1

    for i, (date, title, url) in enumerate(rows, 1):
        print("\t".join([str(i), date, title, url]))
    return 0


if __name__ == "__main__":
    sys.exit(main())
