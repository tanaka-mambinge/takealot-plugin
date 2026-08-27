# Shopping research guide

The goal is not to repeat a star rating. The goal is to give the user a compact, evidence-backed picture of fit, strengths, weaknesses, and uncertainty.

## Chat response style

Make the result easy to scan. Lead with the shortlist or recommendation, then use compact product cards rather than an information-heavy catalogue dump. A useful default card looks like this:

```markdown
### Product name — R price | In stock

![Product image](image-url)

Best for: one sentence describing the fit.
Review snapshot: 4.6/5 from 120 reviews. Positives: quiet and easy to assemble. Watch-out: a few buyers report wobble.

[View it on Takealot](takealot-url)
```

Show one cover image per product when remote images can render in chat. Prefer `takealot product images <plid> --limit 1 --json` and render its absolute `local_path` when possible; this avoids remote CDN preview failures. Add gallery images only when appearance, size, ports, accessories, or other visual details affect the decision. Include the price, sale status when known, stock status, direct link, and a short review snapshot. Mention one or two representative positive, critical, or recent review takeaways; do not paste full reviews or every specification into the first response. Keep the initial shortlist to roughly three to five products when comparing options, and offer deeper details on request. If local image download or rendering is unavailable, keep the product card useful with the Takealot link and state that the image preview could not be displayed.

## Workflow

1. Clarify the user’s use case, budget, must-have features, location, and tolerance for trade-offs.
2. Search Takealot and shortlist products that satisfy the request. Keep the search query and result date in mind.
3. Inspect each candidate’s gallery, description, specifications, warranty, seller, returns, stock, and current price.
4. Capture the total review count and complete five-bucket distribution before sampling reviews.
5. Read representative five-star, one-star, and latest reviews. Sample more than one page when the count is large or themes conflict.
6. Search the exact model externally. Check manufacturer documentation, independent testing, reputable publications, direct Reddit discussions, and credible retailer information.
7. Compare evidence by theme: performance, durability, comfort, compatibility, setup, support, delivery, and value.
8. Produce a balanced brief with explicit confidence and caveats.

## Query patterns

```text
"<brand> <model> review"
"<brand> <model> problems"
site:reddit.com "<brand> <model>"
site:reddit.com/r/<relevant_subreddit> "<model>"
"<model>" teardown OR durability OR battery OR accuracy
"<model>" manual OR specifications OR warranty
```

Use the model number or barcode when a generic product title would create false matches. Search snippets are leads, not evidence; open the underlying page and link it in the brief.

## Link integrity

For every Takealot product card, resolve the item with the CLI and use the returned normalized `url` field. Do not synthesize a route from a PLID or copy a URL from an old search result. The current canonical route may omit `/product/`; if a candidate link contains that legacy segment, re-run `takealot product get <plid> --json` and replace it with the returned URL.

When web verification is available, request the canonical product page before publishing the link and reject a 404. If verification is blocked by a challenge or unavailable, label the link unverified; do not call it a working link. Confirm that the link’s PLID matches the product facts, and keep API and image URLs separate from the clickable product-page link.

## Evidence rules

- Product fields, visible images, rating counts, review dates, and quoted product copy are observations. Label interpretation as interpretation.
- A repeated, specific complaint is more meaningful than one isolated anecdote, but user reviews remain anecdotal.
- Reddit is valuable for long-term ownership and edge cases, not a representative survey. Do not generalize a subreddit thread to every buyer.
- Manufacturer documentation is strongest for specifications and compatibility. Independent testing is strongest for measured performance. Reputable publications can provide useful comparative context.
- Compare reviews within the same variant. A colour, size, generation, bundle, or seller change can make evidence non-comparable.
- Note review count and distribution. A 4.8 average from 12 reviews deserves lower confidence than a 4.6 average from 1,000 reviews.
- Report the date for time-sensitive prices, stock, delivery, promotions, reviews, and external articles.
- Flag contradictions, missing specifications, suspiciously low counts, repeated failure modes, warranty ambiguity, and product images that do not establish a claimed feature.

## Interpreting negative reviews

Treat a negative review as evidence about a particular experience, not an automatic product verdict. First label what kind of problem it describes:

- Preference: taste, firmness, noise, colour, style, or general feel.
- Fit/size: too small, too large, incompatible, or wrong for the reviewer's space or use case.
- Cosmetic/condition: scratches, dents, missing parts, or packaging damage.
- Functional defect: the product does not perform a core advertised function.
- Durability: it wears out, breaks, overheats, or fails prematurely.
- Delivery/support: courier, seller, warranty, or returns problems.

Then check whether the theme repeats, how recent it is, which variant it concerns, and whether the reviewer expected something the listing already discloses. “Too small” can be a subjective fit or expectation issue when dimensions are clear; repeated reports of scratches may indicate packaging or quality-control trouble even if the item works. Distinguish isolated taste from corroborated functional or safety concerns, and say whether the complaint is likely to matter to this user.

Use short quotations only when they add clarity, and otherwise paraphrase. Include the review date when relevant. Never identify reviewers or reproduce a long review.

## Privacy and copyright

Do not output reviewer names, customer IDs, signatures, emails, or other personal data. Summarize review themes and quote only short excerpts when necessary. Do not reproduce whole reviews, articles, or manuals. Link to the original source so the user can read it.

## Brief template

```text
Product overview
  What it is, current Takealot price/stock, variant, rating, review count, and date checked.

What the product appears good at
  The use cases supported by product facts and repeated positive evidence.

Common positive evidence
  Recurring Takealot themes, with review dates where useful.

Common negative evidence
  Recurring complaints, failure modes, limitations, or support issues.

Review distribution and notable complaints
  1★ through 5★ counts, sample size, filters used, and whether variants differ.

Image and description observations
  What several images show, what the description/specifications say, and what remains unverified.

External-source findings
  Direct links to manufacturer docs, independent reviews, Reddit threads, and other relevant sources.

Who should consider it / who should avoid it
  Fit against the user’s stated priorities.

Confidence and caveats
  Evidence quality, date checked, volatility, conflicts, and missing information.

Alternatives
  Only when another candidate better addresses a stated weakness or trade-off.
```
