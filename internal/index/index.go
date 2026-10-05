// Package index demonstrates why indexes exist: it runs the same query
// against an in-memory table twice — once as a full table scan, once via
// a hash index on the queried column — and prints a simulated query-plan
// plus wall-clock timings.
//
// The Planner/Plan types are the port (a query planner), the in-memory
// Table and HashIndex are adapters.
package index

import (
	"fmt"
	"time"
)

// Order is one fictional e-commerce order row.
type Order struct {
	ID         int
	CustomerID string
	TotalCents int
	Status     string
}

// Plan is what a query planner would print: the access path plus how many
// rows it expects to touch.
type Plan struct {
	AccessPath string // e.g. "SCAN" or "INDEX SEEK"
	RowsTouched int
	Detail      string
}

// QueryResult carries the plan, the matched rows, and how long it took.
type QueryResult struct {
	Plan     Plan
	Rows     []Order
	Duration time.Duration
}

// Table is an in-memory table with an optional hash index on CustomerID.
type Table struct {
	name   string
	rows   []Order
	byCust map[string][]int // index: customer_id -> row positions (nil = none)
}

// NewTable creates a table holding n fictional orders spread across
// 1,000 customers.
func NewTable(n int) *Table {
	statuses := []string{"paid", "shipped", "refunded", "pending"}
	t := &Table{name: "orders"}
	for i := 0; i < n; i++ {
		t.rows = append(t.rows, Order{
			ID:         i + 1,
			CustomerID: fmt.Sprintf("cust-%04d", 1000+i%1000),
			TotalCents: 500 + (i*137)%50000,
			Status:     statuses[i%len(statuses)],
		})
	}
	return t
}

// QueryByCustomer scans the table row by row (no index).
func (t *Table) QueryByCustomer(customerID string) QueryResult {
	start := time.Now()
	var out []Order
	for _, r := range t.rows {
		if r.CustomerID == customerID {
			out = append(out, r)
		}
	}
	return QueryResult{
		Plan: Plan{
			AccessPath:  "SCAN",
			RowsTouched: len(t.rows),
			Detail:      fmt.Sprintf("FULL TABLE SCAN on %s: no index on customer_id", t.name),
		},
		Rows:     out,
		Duration: time.Since(start),
	}
}

// BuildIndex creates a hash index on customer_id (one-time cost).
func (t *Table) BuildIndex() time.Duration {
	start := time.Now()
	t.byCust = map[string][]int{}
	for i, r := range t.rows {
		t.byCust[r.CustomerID] = append(t.byCust[r.CustomerID], i)
	}
	return time.Since(start)
}

// QueryByCustomerIndexed answers the same query via the index.
func (t *Table) QueryByCustomerIndexed(customerID string) QueryResult {
	start := time.Now()
	var out []Order
	touched := 0
	if t.byCust == nil {
		return QueryResult{Plan: Plan{AccessPath: "SCAN", Detail: "index not built"}}
	}
	for _, pos := range t.byCust[customerID] {
		touched++
		out = append(out, t.rows[pos])
	}
	return QueryResult{
		Plan: Plan{
			AccessPath:  "INDEX SEEK",
			RowsTouched: touched,
			Detail:      fmt.Sprintf("HASH INDEX SEEK on %s(customer_id): bucket lookup, no table scan", t.name),
		},
		Rows:     out,
		Duration: time.Since(start),
	}
}

// Count returns the row count.
func (t *Table) Count() int { return len(t.rows) }

// PrintPlan prints a query plan the way a database EXPLAIN would.
func PrintPlan(label string, q QueryResult) {
	fmt.Printf("  -- %s --\n", label)
	fmt.Printf("  QUERY PLAN: %s %d rows   (%s)\n", q.Plan.AccessPath, q.Plan.RowsTouched, q.Plan.Detail)
	fmt.Printf("  matched: %d rows, elapsed: %s\n", len(q.Rows), q.Duration)
}
