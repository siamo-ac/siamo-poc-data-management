# Normalized rows vs denormalized document: when each wins

The `docmodel` demo models the same fictional coffee-shop order two ways.
Here is the short rationale for choosing between them in a real design.

## The two models

**Normalized (relational style):** three tables — `customers`, `orders`,
`line_items`. Every fact lives in exactly one place. Rendering one order
needs a 3-table join.

**Denormalized document (NoSQL style):** one JSON document per order with
the customer embedded. Rendering one order is a single key lookup — no joins.

## Normalized rows win when…

- **The same fact is read many ways.** "All orders for customer X", "all
  line items with SKU Y this quarter", "total revenue by city" — ad-hoc
  queries across relationships are what relational engines and their
  optimizers are built for.
- **Updates must stay consistent.** If Alba's Bakery changes its name,
  one row changes. In the document model you'd rewrite every order
  document (or live with stale copies) — a classic update-anomaly trade.
- **Integrity matters more than read speed.** Foreign keys and transactions
  make "an order can't exist without a customer" enforceable, not hopeful.
- **The data is highly relational** — many-to-many links, shared entities
  referenced from lots of places.

Typical fit: core business ledgers, billing, inventory, anything auditors read.

## Denormalized documents win when…

- **Reads follow one access path.** If the app almost always fetches "the
  whole order" (a screen, an API response), one document = one lookup and
  the read path is trivially fast.
- **The unit of change is the document.** Orders are mostly immutable once
  placed — embedding customer details is safe because they rarely change
  after the fact.
- **You shard by the document key.** One document = one shard = no
  cross-shard joins, which is exactly why document stores pair so well
  with the sharding demo in (a).
- **Schema flexibility matters.** Documents can carry different shapes side
  by side while the model evolves, without migrations.

Typical fit: catalogs, user profiles, event payloads, session/cart state,
content pages.

## The honest middle

Real systems mix both: a normalized system of record behind the scenes,
with denormalized read models (or a cache) in front of the hot read paths.
The demo's `docs/when-each-wins.md` companion `README` calls this out —
picking one model for everything is the usual beginner mistake.
