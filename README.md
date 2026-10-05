# POC: Data Management — Sharding, Indexes, Document Modeling

Siamo Workflow Atlas demo portfolio · backend-architecture track (topic: **data management**).
All data in this demo is fictional.

## Concept (plain language)

Databases slow down as they grow. Three ideas keep them fast at scale:

1. **Sharding** — split one big dataset across several machines ("shards")
   by a **shard key** (here: the customer id). Each machine holds a slice of
   the data, so reads and writes scale out instead of piling onto one box.
2. **Indexes** — a lookup structure on a frequently-queried column so the
   engine doesn't read every row to find a few. This demo prints the query
   plan before and after: `SCAN 100000 rows` vs `INDEX SEEK 100 rows`.
3. **Document modeling** — in NoSQL stores you can denormalize: keep
   everything one screen needs in a single document instead of joining
   several normalized tables. Faster reads, trickier updates.

## Run the demo

```bash
cd ~/workspace/services/pocs/siamo-poc-data-management

go build ./...                 # verify it compiles
go run ./cmd/data-mgmt shard     # (a) shard-key routing + rehash on shard add
go run ./cmd/data-mgmt index     # (b) table scan vs indexed lookup
go run ./cmd/data-mgmt docmodel  # (c) normalized rows vs denormalized document

# optional tiny HTTP view of the shard router:
go run ./cmd/data-mgmt serve
curl 'localhost:8090/shard?key=cust-1001'   # which shard holds this key?
curl 'localhost:8090/distribution'           # records per shard
```

## What to observe

- **(a) shard:** with plain modulo routing, adding a 4th shard moves ~7 of 8
  demo keys — nearly everything is re-located (a mass migration in real
  life). With consistent hashing, only the new shard's share of 2,000 keys
  moves; the rest stay put. That's the core argument for consistent hashing
  (and why real systems use it for caches and distributed stores).
- **(b) index:** the same query runs as `SCAN 100000 rows` (~6 ms) without
  an index and `INDEX SEEK 100 rows` (~30 µs) with one — same answer, ~200×
  faster, and the gap grows with table size. Note the one-time index build
  cost: indexes speed reads but cost time on every write.
- **(c) docmodel:** one order rendered from 3 normalized tables (join) vs
  one document (single key lookup). See `docs/when-each-wins.md` for which
  model wins when — the honest answer is "usually both, in different places".
- **serve:** the router is a pure function of the key — the same key always
  lands on the same shard, which is what makes sharding safe to reason about.

## Honest limits (POC — not production hardening)

- In-memory everything: no persistence, no replication, no failover. Restart
  and the data is gone.
- The "query planner" is simulated print output, not a real cost-based
  optimizer; timings are wall-clock on this machine, not a benchmark.
- Modulo/consistent-hash demos use FNV-1a; real systems pick hash functions
  and vnode counts to match their key distribution and rebalance budget.
- No cross-shard query support, no hot-shard mitigation (a celebrity customer
  would still overload one shard), no transactional guarantees across shards.
- Single process — no network partitions, no clock skew, no consensus.
