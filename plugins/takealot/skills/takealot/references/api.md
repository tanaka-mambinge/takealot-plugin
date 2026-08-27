# Takealot API reference

This is an implementation note for the CLI, not an official Takealot API contract. Agents must use the downloaded `takealot` binary and must not recreate these requests.

## Public catalogue

```text
GET https://api.takealot.com/rest/v-1-14-0/searches/products,filters,facets,sort_options,breadcrumbs,slots_audience,context,seo,layout
GET https://api.takealot.com/rest/v-1-16-0/product-details/PLID{plid}
GET https://api.takealot.com/rest/v-1-16-0/product-reviews/plid/{plid}
```

The CLI normalizes separate `plid`, `product_id`, and `tsin` fields. Detail/review inputs accept a numeric PLID or a full Takealot URL containing `/PLID...`; ambiguous product IDs and TSINs are rejected.

## Android authentication

The installed Android app uses `POST /customers/login` on `v-1-16-0` with JSON sections for `customer_login` fields `email`, `password`, and `captcha`. A second request may add `two_step_verification` fields `otp` and `trust_this_device`; the HTTP cookie jar must retain the first response's Cloudflare cookie. Successful `auth_info` contains a JWT, refresh token, CSRF token, tracking ID, customer ID, and related mobile credentials.

Refresh uses:

```text
POST /customers/auth/refresh
Authorization: Bearer {jwt}
{"platform":"android","refresh_token":"{refresh_token}","tracking_id":"{tracking_id}"}
```

The CLI stores the session payload in the native OS keyring and never emits token fields in normalized output.

## Wishlist routes copied from the Android app

All routes use `https://api.takealot.com/rest/v-1-16-0` and the authenticated customer ID from the keyring session.

```text
GET    /customers/{customer_id}/wishlists
POST   /customers/{customer_id}/wishlists                 {"name":"..."}
PUT    /customers/{customer_id}/wishlists/{group_id}      {"name":"..."}
DELETE /customers/{customer_id}/wishlists/{group_id}

GET    /customers/{customer_id}/wishlists/{group_id}/items
PUT    /customers/{customer_id}/wishlists/items/pid/{product_id}
       {"reset":false,"groups":[{group_id}]}
DELETE /customers/{customer_id}/wishlists/items/pid/{product_id}
```

The CLI resolves a PLID/URL through product details before using the `pid` route. This prevents a TSIN from being silently treated as a product ID. Wishlist mutations are never implicit and require the CLI's `--confirm` flag after chat confirmation.

## Privacy and failures

Reviewer identity fields and session secrets are excluded from normalized output. The CLI categorizes 401/403/404/429 and Cloudflare responses, refreshes a session only once after a 401, and never falls back to unauthenticated direct API requests when the binary or session is unavailable.
