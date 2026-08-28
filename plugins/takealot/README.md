# Takealot Shopping Assistant

A Codex-compatible Takealot shopping assistant plugin with a cross-platform Go CLI. Catalogue research is read-only; authenticated wishlist changes are explicit, opt-in commands. The agent-facing behavior lives in [`skills/takealot/SKILL.md`](skills/takealot/SKILL.md); this README contains implementation and API notes for developers.

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
takealot auth status --json
takealot auth login --email user@example.com --password-stdin
takealot wishlist list --json
takealot wishlist add <group-id> <plid-or-takealot-url> --confirm
```

The CLI emits human-readable output by default and stable normalized JSON with `--json`. It accepts numeric PLIDs, `PLID123`, and Takealot product URLs. Product IDs and TSINs remain separate fields and are not accepted as ambiguous product references.

Product URLs are normalized to Takealot’s current canonical route. Consumers should use the returned `url` field rather than constructing links from PLIDs or copying stale `/product/` routes.

`takealot product images` downloads up to 10 gallery images for local rendering. By default, files are stored in the hidden platform-independent cache directory `~/.takealot/images/<PLID>` using Go’s user-home and filepath APIs. On Windows, the default directories are also marked with the Windows Hidden attribute. Use `--dir` to select another directory. JSON output includes the absolute cache directory, original image URL, local path, content type, and byte count. The command only downloads image URLs returned by Takealot product details and keeps each response below 16 MiB.

The CLI does not include credentials. `auth login` follows the Android customer login flow and saves the resulting session in the OS keyring (Linux Secret Service, macOS Keychain, or Windows Credential Manager). `auth status` never prints secrets. Wishlist commands use the authenticated mobile routes; all mutations require `--confirm`. Cart, checkout, payment, order, and other account mutations remain out of scope.

By default, `auth login` starts a loopback-only, one-time login page and attempts to open it with the native browser launcher. The page posts credentials to the local CLI, not to chat or a remote service. Use `--password-stdin` only for automation; its first input line is the password and its optional second line is the OTP.

## Release and agent bootstrap

The CLI is not installed as part of the plugin. The Takealot skill runs the native launcher before doing research. The launcher checks the official latest release tag and reuses the cached binary only when it matches that tag; otherwise it downloads and verifies a replacement:

- Linux/macOS: `scripts/download_cli.sh`, using `uname`, `curl`, `mktemp`, `awk`, and `sha256sum` or `shasum`.
- Windows: `scripts/download_cli.ps1`, using PowerShell's `Invoke-WebRequest` and `Get-FileHash`.

Network operations use a 30-second timeout by default. The agent automatically retries a timed-out read-only operation once with `TAKEALOT_HTTP_TIMEOUT_SECONDS=90`; users do not need to approve that retry. Set `TAKEALOT_HTTP_TIMEOUT_SECONDS` to an integer from `1` through `90` when a slower operation needs more time; values above 90 are rejected. Wishlist mutations are not blindly repeated after a timeout because the server may already have accepted them.

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

This plugin uses Takealot catalogue, review, authentication, and wishlist routes discovered during local research. These customer/mobile endpoints are not presented as an official Takealot developer contract and may change. See [`TAKEALOT-API-RESEARCH.md`](TAKEALOT-API-RESEARCH.md) for the full exploration notes.

| Purpose | Base | Route |
| --- | --- | --- |
| Search | `https://api.takealot.com/rest/v-1-14-0` | `/searches/products,filters,facets,sort_options,breadcrumbs,slots_audience,context,seo,layout` |
| Product details | `https://api.takealot.com/rest/v-1-16-0` | `/product-details/PLID{plid}` |
| Product reviews | `https://api.takealot.com/rest/v-1-16-0` | `/product-reviews/plid/{plid}` |

Search uses the observed search-instance parameters, `qsearch`, and `searchbox=true`. Details use `platform=android`, `show_takealot_now_alt=false`, and `offer_opt=true`. Catalogue commands make only GET requests; authenticated wishlist commands use the Android mobile session and wishlist routes.

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
