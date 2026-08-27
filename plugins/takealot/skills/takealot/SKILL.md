---
name: takealot
description: Research Takealot products with catalogue data, images, reviews, and external sources to produce balanced shopping briefs.
---

# Takealot Shopping Assistant

Use the downloaded `takealot` CLI as the only interface for Takealot catalogue, product, review, image, authentication, and wishlist operations. Read [the shopping research guide](references/shopping-research.md) before doing product research; it contains the detailed evidence rules, negative-review interpretation, external-search patterns, and brief template.

Catalogue research is read-only. Wishlist changes are allowed only after the user explicitly confirms the exact action. Never add to cart, check out, pay, place an order, or perform other account actions.

## Required rules

- Bootstrap the latest verified CLI binary before any Takealot operation. Never assume `takealot` is on `PATH`.
- Never call Takealot APIs directly, reconstruct requests, scrape the storefront, or substitute web snippets for CLI results.
- Keep PLID, product ID, and TSIN separate. Detail and review commands accept numeric PLIDs or full Takealot URLs only.
- Use the exact canonical `url` returned by `product get`; never construct a product link. If verification is available, check it is not a 404.
- Do not claim location-specific availability, stock, delivery, shipping, or fulfilment. Prices, promotions, and listing details are volatile.
- Never print, request in chat, or store passwords/tokens outside the CLI's secure local flow. Never read the mobile app's private storage.
- Keep responses compact: use product cards, short review takeaways, clickable links, and local image previews. Do not dump raw JSON or full descriptions/reviews.

## Bootstrap the CLI

The plugin does not include or require a binary. Use the bundled native launcher and keep the returned absolute path in `TAKEALOT_BIN` for the whole task:

```bash
TAKEALOT_BIN="$(sh <plugin-root>/scripts/download_cli.sh)"
```

```powershell
$TAKEALOT_BIN = & powershell -NoProfile -ExecutionPolicy Bypass -File <plugin-root>\scripts\download_cli.ps1
```

The launcher detects OS/architecture, downloads the matching latest GitHub Release asset, verifies `checksums.txt`, atomically updates the hidden user cache, and prints the executable path. It uses no Python, Go, `jq`, `gh`, package manager, global install, or `PATH` change. If it fails, explain the platform/download/checksum error and stop the Takealot portion; do not bypass verification or fall back to direct API calls.

When the CLI prints a localhost login URL, immediately repeat the exact URL in chat as a clickable Markdown link and keep the login command running while the user completes it.

## CLI operations

Use normalized JSON when selecting or comparing products:

```bash
"$TAKEALOT_BIN" version --json
"$TAKEALOT_BIN" search "wireless earbuds" --limit 10 --json
"$TAKEALOT_BIN" product get <plid-or-takealot-url> --json
"$TAKEALOT_BIN" product images <plid-or-takealot-url> --limit 3 --json
"$TAKEALOT_BIN" product reviews <plid-or-takealot-url> --rating 5 --sort helpful --json
"$TAKEALOT_BIN" product reviews <plid-or-takealot-url> --rating 1 --sort latest --json
"$TAKEALOT_BIN" product reviews <plid-or-takealot-url> --sort latest --json
```

For each serious candidate, fetch details, download up to three images, and fetch the overall rating/distribution plus representative five-star, one-star, and latest reviews. View the local images when image viewing is available. Follow the research guide for what to inspect, how to compare variants, and how to weigh anecdotal reviews against external evidence.

## Product links and images

For every product link, resolve with `product get` and copy only its normalized `url` field. The link must contain the matching PLID. Never use stale search snippets, browser history, API URLs, or remote image URLs as the product link.

For every shortlisted or recommended product, run:

```bash
"$TAKEALOT_BIN" product images <plid-or-url> --limit 3 --json
```

Render at least one returned absolute `local_path` immediately below the product name/link:

```markdown
![Product image](/absolute/path/01.jpg)
```

Use two or three images when they clarify size, build, ports, controls, or included accessories. If downloading or rendering fails, say the preview is unavailable and retain the verified product link; do not pretend to have viewed the image.

## Login and wishlists

Only use account commands when the user asks. The CLI follows the Android login flow, handles OTP and token refresh, and stores the session in the native OS keyring:

```bash
"$TAKEALOT_BIN" auth status --json
"$TAKEALOT_BIN" auth login
"$TAKEALOT_BIN" auth login --email user@example.com
"$TAKEALOT_BIN" auth logout
"$TAKEALOT_BIN" wishlist list --json
"$TAKEALOT_BIN" wishlist items <group-id> --json
"$TAKEALOT_BIN" wishlist add <group-id> <plid-or-url> --confirm --json
"$TAKEALOT_BIN" wishlist remove <plid-or-url> --confirm --json
```

Default login opens a temporary loopback page for credentials and OTP. Never put credentials in command arguments or chat. Before any wishlist mutation, show the exact canonical product link and wishlist name/group, ask for confirmation in chat, then pass `--confirm`. Do not modify wishlists as a side effect of research.

## Response shape

Lead with the recommendation or shortlist. Use compact cards containing the product name/link, local image, current listed price and sale status when evident, why it fits, and a small rating/review snapshot. Then provide the relevant evidence and caveats. Read the guide for the full shopping-brief structure and source-quality rules.
