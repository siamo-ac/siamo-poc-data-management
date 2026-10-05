// Command data-mgmt is the demo CLI for the data-management POC.
// Subcommands: shard | index | docmodel | serve
package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/siamo-systems/siamo-poc-data-management/internal/docmodel"
	"github.com/siamo-systems/siamo-poc-data-management/internal/index"
	"github.com/siamo-systems/siamo-poc-data-management/internal/shard"
)

var demoKeys = []string{
	"cust-1001", "cust-1002", "cust-1003", "cust-1004",
	"cust-1005", "cust-1006", "cust-1007", "cust-1008",
}

var demoNames = map[string]string{
	"cust-1001": "Alba's Bakery", "cust-1002": "Harbor Coffee",
	"cust-1003": "Green Grocer", "cust-1004": "Blue Bike Shop",
	"cust-1005": "Cedar Books", "cust-1006": "Maple Diner",
	"cust-1007": "Pine Pharmacy", "cust-1008": "Oak Outfitters",
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	switch os.Args[1] {
	case "shard":
		demoShard()
	case "index":
		demoIndex()
	case "docmodel":
		demoDocmodel()
	case "serve":
		serve()
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Println("data-mgmt — data management POC demos (all fictional data)")
	fmt.Println("  shard     shard-key routing + what happens when a shard is added")
	fmt.Println("  index     table scan vs indexed lookup, with query plans + timings")
	fmt.Println("  docmodel  normalized rows vs denormalized document")
	fmt.Println("  serve     tiny HTTP demo of the shard router (GET /shard?key=...)")
}

// ---------------------------------------------------------------------------

func demoShard() {
	fmt.Println("=== (a) Shard-key routing ===")
	fmt.Println("Modulo router, 3 shards. Each key's shard is hash(key) % 3:")
	mod := shard.NewModuloRouter(3)
	store := shard.NewStore(mod)
	for _, k := range demoKeys {
		sh := store.Put(shard.Record{Key: k, Name: demoNames[k], City: "Stamford"})
		fmt.Printf("  %-10s -> %s\n", k, sh)
	}

	fmt.Println("\nNow add a 4th shard to the modulo router. Because the modulo")
	fmt.Println("base changed (3 -> 4), hash(key) % 4 re-points most keys:")
	mod4 := mod.AddShard()
	moved := 0
	for _, k := range demoKeys {
		before := mod.ShardFor(k)
		after := mod4.ShardFor(k)
		mark := "stays"
		if before != after {
			mark = "MOVES"
			moved++
		}
		fmt.Printf("  %-10s %s -> %s   [%s]\n", k, before, after, mark)
	}
	fmt.Printf("\n  modulo rehash: %d of %d keys moved — nearly everything is\n", moved, len(demoKeys))
	fmt.Println("  re-located. In production that's a mass data migration.")

	fmt.Println("\n--- Consistent hashing (hash ring, 128 vnodes/shard) ---")
	fmt.Println("Using 2,000 keys to show the distribution (8 keys cluster by luck):")
	r3 := shard.NewHashRing(3, 128)
	r4 := shard.NewHashRing(3, 128)
	r4.AddShard("shard-3")
	dist3 := map[string]int{}
	dist4 := map[string]int{}
	moved = 0
	const ringN = 2000
	for i := 0; i < ringN; i++ {
		k := fmt.Sprintf("ring-key-%04d", i)
		b, a := r3.ShardFor(k), r4.ShardFor(k)
		dist3[b]++
		dist4[a]++
		if b != a {
			moved++
		}
	}
	fmt.Printf("  3 shards: %v\n", dist3)
	fmt.Printf("  4 shards: %v\n", dist4)
	fmt.Printf("\n  consistent hashing: only %d of %d keys moved (~%.0f%%, the share\n",
		moved, ringN, float64(moved)/float64(ringN)*100)
	fmt.Println("  the new shard takes). The rest never leave their shard.")
}

func demoIndex() {
	fmt.Println("=== (b) Table scan vs indexed lookup ===")
	t := index.NewTable(100000)
	fmt.Printf("Table orders: %d fictional rows.\n", t.Count())
	fmt.Println("Query: SELECT * FROM orders WHERE customer_id = 'cust-1042'")

	fmt.Println("1) No index:")
	q1 := t.QueryByCustomer("cust-1042")
	index.PrintPlan("before", q1)

	fmt.Println("\nBuilding a hash index on customer_id...")
	cost := t.BuildIndex()
	fmt.Printf("  index build cost (one-time): %s\n", cost)

	fmt.Println("\n2) With index:")
	q2 := t.QueryByCustomerIndexed("cust-1042")
	index.PrintPlan("after", q2)

	fmt.Printf("\nSame answer (%d rows), but the plan touches %d rows instead of\n", len(q2.Rows), q2.Plan.RowsTouched)
	fmt.Printf("%d — this is why a missing index turns a millisecond lookup into\n", q1.Plan.RowsTouched)
	fmt.Println("a full-table scan that gets slower as the table grows.")
}

func demoDocmodel() {
	fmt.Println("=== (c) Normalized rows vs denormalized document ===")
	n := docmodel.NewNormalized()
	v, _ := n.OrderView(1)
	fmt.Println("Normalized (relational style):")
	fmt.Println("  " + v)

	d := docmodel.NewDocument()
	v2, _ := d.OrderDoc(1)
	fmt.Println("\nDenormalized document (NoSQL style):")
	fmt.Println("  " + v2)

	j, _ := d.OrderDocJSON(1)
	fmt.Println("\nThe document as stored (single JSON value under one key):")
	fmt.Println(j)
	fmt.Println("\nSee docs/when-each-wins.md for the trade-off rationale.")
}

// ---------------------------------------------------------------------------

func serve() {
	store := shard.NewStore(shard.NewModuloRouter(3))
	for _, k := range demoKeys {
		store.Put(shard.Record{Key: k, Name: demoNames[k], City: "Stamford"})
	}
	http.HandleFunc("/shard", func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("key")
		if key == "" {
			http.Error(w, `{"error":"pass ?key=cust-1001"}`, http.StatusBadRequest)
			return
		}
		rec, sh, ok := store.Get(key)
		if !ok {
			http.Error(w, `{"error":"unknown key"}`, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"key":%q,"shard":%q,"name":%q}`, key, sh, rec.Name)
	})
	http.HandleFunc("/distribution", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, "%v", store.Distribution())
	})
	fmt.Println("serving shard demo on :8090 — try: curl 'localhost:8090/shard?key=cust-1001'")
	_ = http.ListenAndServe(":8090", nil)
}
