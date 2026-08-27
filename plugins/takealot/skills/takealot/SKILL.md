---
name: takealot
description: Research Takealot products with catalogue data, images, reviews, and external sources to produce balanced shopping briefs.
---

# Takealot Shopping Assistant

Use the `takealot` CLI and web research to help a user decide whether a product is worth considering. This skill is read-only: it may search the catalogue, inspect public product data, inspect public reviews, view product images, and research external sources. It must never log in, access customer accounts, add to cart, check out, pay, place an order, or change any Takealot state.

## Source priority and availability boundaries

Use the CLI as the primary source for all Takealot catalogue facts. After bootstrapping it, use the CLI first for Takealot search results, product identity, listed price, sale status, ratings, review counts, review text, product details, images, and canonical product links. Do not use web search snippets or a Takealot page as a substitute for CLI data, and do not reconstruct the API request yourself. Use web research after the CLI for Reddit, manufacturer documentation, independent reviews, and other external evidence.

If the CLI cannot run, explain the failure and do not claim a Takealot price, rating, product detail, or listing status from web results alone. External research may still be labelled separately when useful.

Do not give an availability verdict. Takealot availability, stock, delivery, shipping, and fulfilment can vary by delivery address, seller, region, and time. Never say that an item is “available”, “in stock”, “out of stock”, “unavailable”, or guaranteed to arrive. You may report the listed price and sale state returned by the CLI, with the date checked, but do not turn catalogue data into a location-specific availability claim.

## CLI bootstrap (required)

The CLI is not installed when this plugin is installed. Before any Takealot research, always run the native launcher. It reuses the verified cached binary when it is current and only downloads a release binary when the cache is missing or a newer release is available.

Use the bundled native launcher scripts:

```bash
# Linux or macOS; replace <plugin-root> with the installed plugin directory.
TAKEALOT_BIN="$(sh <plugin-root>/scripts/download_cli.sh)"
```

```powershell
# Windows PowerShell; replace <plugin-root> with the installed plugin directory.
$TAKEALOT_BIN = & powershell -NoProfile -ExecutionPolicy Bypass -File <plugin-root>\scripts\download_cli.ps1
```

The launcher performs a lightweight latest-release check, compares the release tag with its hidden cache marker, and only then downloads the matching asset and `checksums.txt`. New downloads are SHA-256 verified and atomically replace the cached executable; an up-to-date cache is returned immediately. Use the printed absolute path for every CLI command in the current task. Do not assume `takealot` is on `PATH`, do not install it globally, and do not use Python, Go, `jq`, `gh`, or a package manager on the user's machine. The hidden cache is `~/.takealot/bin/takealot` plus `~/.takealot/bin/takealot.version` on Unix and `%USERPROFILE%\.takealot\bin\takealot.exe` plus `%USERPROFILE%\.takealot\bin\takealot.version` on Windows.

If the launcher fails, report the platform, download, release, or checksum error clearly and stop the Takealot CLI portion of the task. Do not bypass checksum verification, use an unverified binary, reconstruct the Takealot API request yourself, or pretend that catalogue research was completed.

## CLI commands

```bash
"$TAKEALOT_BIN" version --json
"$TAKEALOT_BIN" search "wireless earbuds" --limit 10 --json
"$TAKEALOT_BIN" product get 66383997 --json
"$TAKEALOT_BIN" product get "https://www.takealot.com/.../PLID66383997" --json
"$TAKEALOT_BIN" product images 66383997 --limit 3 --json
"$TAKEALOT_BIN" product reviews 66383997 --rating 5 --sort helpful --page 0 --json
"$TAKEALOT_BIN" product reviews 66383997 --rating 1 --sort latest --json
"$TAKEALOT_BIN" product reviews 66383997 --sort latest --page 0 --variant Black --json
```

Use PLIDs or full Takealot product URLs for detail and review commands. Keep PLID, product ID, and TSIN distinct; do not guess which identifier a bare number represents. Default output is human-readable; `--json` is normalized JSON and intentionally omits reviewer names, customer IDs, and signatures.

## Product-link integrity

Never construct a Takealot product URL from a title, PLID, product ID, TSIN, or remembered URL pattern. Before presenting a product link:

1. Resolve the product with `takealot product get <plid-or-url> --json`.
2. Use the exact normalized `url` field returned by that command.
3. Confirm that the URL contains the same PLID as the product being described and does not use a stale `/product/` route when the CLI has returned a canonical route without it.
4. When page verification is available, request the canonical URL and confirm it is not a 404. A failed or blocked verification is not evidence about location-specific availability; report the link as unverified instead.

Do not copy links from stale search snippets, old answers, API endpoints, image URLs, or browser history. If a URL returns 404, re-resolve it by PLID and replace it before responding. If no working product page can be confirmed, say so rather than giving the user a link likely to fail.

### Images are required in the response

Product images are part of the shopping answer, not optional metadata. For every shortlisted or recommended product, always run:

```bash
"$TAKEALOT_BIN" product images <plid-or-url> --limit 3 --json
```

Then visibly render at least one returned `local_path` in the product card using an absolute-path Markdown image tag:

```markdown
![Product image](/absolute/path/01.jpg)
```

When three images are available, render two or three if they add useful views of the product. Put the image immediately below the product name/link so the user sees the product before reading the evidence. Never replace an available image with plain text, a raw image URL, or a sentence saying that images are available. Use the local paths returned by the CLI rather than remote `image_urls`; these files are downloaded into the hidden, platform-neutral cache directory `~/.takealot/images/<PLID>`.

If image downloading fails, the returned list is empty, or the chat cannot render local files, say briefly that the image preview is unavailable and still provide the verified clickable Takealot link. Do not pretend an image was viewed, and do not paste a long list of image URLs. Do not download the full gallery unless it is needed.

## Keep the chat useful

Lead with the answer and keep the first response light. For each shortlisted product, present a compact product card with:

- Product name and a clickable Takealot link.
- At least one visibly rendered local product image from `takealot product images`; show two or three when they add useful visual context.
- Current listed price and currency, sale status when evident, and USD conversion only when requested.
- A one-sentence explanation of why it fits the user's use case.
- A small review snapshot: average rating, total count, compact 1–5-star distribution, and one positive plus one critical or recent review takeaway.

Use Markdown image tags and links in the response. Do not paste raw JSON, long image URL lists, full descriptions, or a large block of reviews into the first response. Offer to expand the specifications, gallery, or review evidence if the user wants more detail. If images cannot be rendered, say so briefly and provide the product link instead.

## Required research workflow

```mermaid
flowchart TD
    A[User asks about a product or category] --> B[Search Takealot]
    B --> C[Select relevant products]
    C --> D[Fetch product details]
    D --> E[View several gallery images]
    D --> F[Read description specifications warranty seller and returns]
    D --> G[Read rating average count and 1 to 5 star distribution]
    G --> H[Fetch five-star one-star and latest reviews]
    H --> I[Search Reddit and other external sources]
    E --> J[Compare evidence]
    F --> J
    H --> J
    I --> J
    J --> K[Produce a balanced shopping brief]
```

For a category request, search first and choose a small set of genuinely relevant candidates. For each serious candidate:

1. Fetch product details and record PLID, product ID, TSIN, title, brand, listed price, URL, seller, returns, warranty, specifications, description, bullets, variants, and gallery image URLs. Do not report stock or availability.
2. Run `takealot product images <plid-or-url> --limit 3 --json` and render the returned local image paths in the response. View several product images when image viewing is available. Note visible build quality, size, ports, controls, included accessories, packaging, fit, and any mismatch between the images and the written description. Do not infer hidden technical properties from an image.
3. Read the description, product attributes, warranty language, seller information, and exchange/return information. Call out missing or ambiguous specifications.
4. Record the overall average rating, total review count, and every 1–5-star distribution bucket. A high average with few reviews is weaker evidence than a similar average with a large count.
5. Fetch representative five-star and one-star reviews, plus the latest reviews. Use pagination when the first page is not enough to understand repeated themes. When variants exist, check the relevant variant filter and do not merge evidence across materially different variants. Include one or two short snippets or faithful paraphrases in the response rather than dumping full review pages.
6. Search externally using the exact product title, brand, model, barcode, and distinctive identifying terms. Prefer direct Reddit threads, manufacturer documentation, independent reviews, repair or testing publications, and reputable retailers over search snippets.
7. Separate observed facts from interpretation. Treat Takealot reviews and Reddit as useful but anecdotal. Look for repeated, specific complaints and positive themes rather than relying on one unusually enthusiastic or angry review.

### Interpret negative reviews carefully

A single bad review does not make a product bad, but it should not be dismissed. Classify the complaint before drawing a conclusion:

- `Preference`: taste, feel, noise, firmness, colour, or style.
- `Fit/size`: too small, too large, incompatible, or unsuitable for the reviewer's space or use case.
- `Cosmetic/condition`: scratches, dents, missing parts, or packaging damage; treat repeated reports as a possible quality-control or shipping problem.
- `Functional defect`: does not work as advertised, fails to connect, breaks, or cannot perform the core task.
- `Durability`: recurring reports of early failure, wear, or weak construction.
- `Delivery/support`: courier, seller, warranty, or returns experience.

Check frequency, recency, variant, reviewer use case, and corroboration from other reviews. A complaint such as “too small” may be an expectation mismatch when the listed dimensions are clear; “arrived scratched” is a condition issue even if the desk works well. Explain whether the complaint is likely to matter to this user, and distinguish an isolated preference from a repeated functional or safety concern.

## External research

Use queries such as:

```text
"<brand> <model> review"
"<brand> <model> problems"
site:reddit.com "<brand> <model>"
"<model>" battery life OR durability OR warranty
"<model>" site:manufacturer.example
```

Open the direct source and cite clickable links for meaningful external findings. Say when web search is unavailable. In that case provide a clearly labelled Takealot-only assessment rather than implying that external research was completed.

## Shopping brief format

Use these sections when enough evidence is available. Keep the default answer compact: a few product cards and concise evidence, with deeper detail only when useful or requested.

- Product overview
- What the product appears good at
- Common positive evidence
- Common negative evidence
- Review distribution and notable complaints
- Image and description observations
- External-source findings
- Who should consider it
- Who should avoid it
- Confidence and caveats
- Alternatives when useful

Mention the dates of important reviews and external sources. Prices, promotions, and listing details can change. Do not make availability or delivery claims. Flag conflicting evidence, suspiciously low review counts, variant differences, repeated complaints, missing warranty details, and claims that cannot be verified. Never present a reviewer's identity or private customer identifier.

Read [the shopping research guide](references/shopping-research.md) when you need evidence-handling rules or concise brief examples. Use the CLI as the interface to Takealot; do not reconstruct API requests yourself when a CLI command provides the needed data.
