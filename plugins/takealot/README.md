# Takealot Shopping Assistant

A Codex-compatible, read-only shopping assistant plugin with a cross-platform Go CLI. The agent-facing behavior lives in [`skills/takealot/SKILL.md`](skills/takealot/SKILL.md); this README contains implementation and API notes for developers.

The plugin uses the downloaded Takealot favicon in `assets/takealot-icon.png` for both its logo and composer icon. Takealot branding remains the property of Takealot.

## CLI

```bash
takealot search "wireless earbuds" --limit 10 --json
takealot product get 66383997 --json
takealot product get "https://www.takealot.com/.../PLID66383997" --json
takealot product images 66383997 --limit 1 --json
takealot product reviews 66383997 --rating 5 --sort helpful --page 0 --json
takealot product reviews 66383997 --rating 1 --sort latest --json
takealot product reviews 66383997 --sort latest --page 0 --variant Black --json
takealot version
takealot version --json
takealot --version
```

The CLI emits human-readable output by default and stable normalized JSON with `--json`. It accepts numeric PLIDs, `PLID123`, and Takealot product URLs. Product IDs and TSINs remain separate fields and are not accepted as ambiguous product references.

Product URLs are normalized to Takealot’s current canonical route. Consumers should use the returned `url` field rather than constructing links from PLIDs or copying stale `/product/` routes.

`takealot product images` downloads up to 10 gallery images for local rendering. By default, files are stored in the hidden platform-independent cache directory `~/.takealot/images/<PLID>` using Go’s user-home and filepath APIs. On Windows, the default directories are also marked with the Windows Hidden attribute. Use `--dir` to select another directory. JSON output includes the absolute cache directory, original image URL, local path, content type, and byte count. The command only downloads image URLs returned by Takealot product details and keeps each response below 16 MiB.

V1 is intentionally read-only. There are no credentials, persistent authentication, customer-account, cart, checkout, payment, order, or other state-changing operations.

## Release and agent bootstrap

The CLI is not installed as part of the plugin. The Takealot skill runs the native launcher before doing research. The launcher checks the latest release tag and reuses the verified cached binary when it is current; it downloads and verifies a replacement only when the cache is missing or a newer release is available:

- Linux/macOS: `scripts/download_cli.sh`, using `uname`, `curl`, `mktemp`, `awk`, and `sha256sum` or `shasum`.
- Windows: `scripts/download_cli.ps1`, using PowerShell's `Invoke-WebRequest` and `Get-FileHash`.

The launcher checks the stable asset for the host OS/architecture from `tanaka-mambinge/takealot-plugin`, compares the release tag with a hidden cache marker, and verifies `checksums.txt` only when an update is needed. It stores the executable and release marker in a hidden user-scoped cache without changing `PATH`:

```text
Unix:    ~/.takealot/bin/takealot (+ takealot.version)
Windows: %USERPROFILE%\.takealot\bin\takealot.exe (+ takealot.version)
```

No Python, Go runtime, `jq`, `gh`, package manager, or global installation is required on the user's machine. If a download or checksum fails, the skill reports the failure and does not fall back to direct API calls.

GitHub Actions publishes the following assets when a matching semver tag is pushed:

```text
takealot_linux_amd64
takealot_linux_arm64
takealot_darwin_amd64
takealot_darwin_arm64
takealot_windows_amd64.exe
takealot_windows_arm64.exe
checksums.txt
```

To publish a release, update the plugin version, commit the change, and push a matching tag such as `v0.1.0`. The workflow runs tests, builds all targets with embedded version metadata, generates SHA-256 checksums, and creates the GitHub Release.

## API endpoint map

This plugin uses public, read-only catalogue and review routes discovered during local research. These endpoints are not presented as an official Takealot developer contract and may change. See [`TAKEALOT-API-RESEARCH.md`](TAKEALOT-API-RESEARCH.md) for the full exploration notes.

| Purpose | Base | Route |
| --- | --- | --- |
| Search | `https://api.takealot.com/rest/v-1-14-0` | `/searches/products,filters,facets,sort_options,breadcrumbs,slots_audience,context,seo,layout` |
| Product details | `https://api.takealot.com/rest/v-1-16-0` | `/product-details/PLID{plid}` |
| Product reviews | `https://api.takealot.com/rest/v-1-16-0` | `/product-reviews/plid/{plid}` |

Search uses the observed search-instance parameters, `qsearch`, and `searchbox=true`. Details use `platform=android`, `show_takealot_now_alt=false`, and `offer_opt=true`. The client supplies browser/mobile-style headers and makes only GET requests.

## Normalized data

Search results expose PLID, product ID, TSIN, title, brand, URL, price, stock, gallery image URLs, and rating summary. Product details add description text/HTML, bullet points, product attributes, variants, seller data when available, returns information, and the complete gallery. Image templates containing `{size}` are normalized to `s-full.file` URLs so they can be viewed directly.

Downloaded image output is separate from product details and is intended for agents that can render local files. It returns `local_path` values rather than embedding binary data in JSON, so the agent can use an absolute Markdown image path without exposing raw upstream image responses.

Rating summaries include the average, total count, and complete 1–5-star distribution. Review output includes page metadata, sort/filter definitions, rating, review text, date, upvotes, time-after-purchase where available, and variant information. Reviewer names, customer IDs, and signatures are deliberately removed.

## Review controls

- `page` is zero-based; the default is page 0.
- `rating=1` through `rating=5` filters by star rating.
- The default sort is most helpful; latest uses upstream `sort=SO_LATEST`.
- Variant filtering uses the product’s available variant value, for example `colour_variant=Black`.
- Use `page_info.total` and `page_info.total_pages` to understand coverage.

## Errors and limits

The CLI reports clear categories for invalid references, empty results, malformed JSON, `403`, `404`, `429`, and Cloudflare challenge pages. A `Retry-After` value is shown when supplied. Do not bypass a Cloudflare challenge or retry aggressively. Prices, stock, delivery, promotions, and review totals are volatile.

## Development

```bash
gofmt -w .
go test ./...
go build ./cmd/takealot
GOOS=linux GOARCH=amd64 go build -o /tmp/takealot-linux-amd64 ./cmd/takealot
GOOS=darwin GOARCH=arm64 go build -o /tmp/takealot-darwin-arm64 ./cmd/takealot
GOOS=windows GOARCH=amd64 go build -o /tmp/takealot-windows-amd64.exe ./cmd/takealot
python3 /home/t12e/.codex/skills/.system/plugin-creator/scripts/validate_plugin.py .
```
