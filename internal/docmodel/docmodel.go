// Package docmodel shows the same fictional domain modeled two ways:
// as normalized rows (orders + line_items + customers) and as one
// denormalized document per order. Each model has a Repository port; the
// in-memory structs are adapters.
package docmodel

import (
	"encoding/json"
	"fmt"
)

// ---------------------------------------------------------------------------
// Shared domain
// ---------------------------------------------------------------------------

// LineItem is one line on an order.
type LineItem struct {
	SKU      string  `json:"sku"`
	Name     string  `json:"name"`
	Quantity int     `json:"qty"`
	UnitCents int    `json:"unit_cents"`
}

// ---------------------------------------------------------------------------
// Model A — normalized rows (relational style)
// ---------------------------------------------------------------------------

// NCustomer, NOrder, NLineItem are three normalized tables.
type NCustomer struct {
	ID   string
	Name string
	City string
}

type NOrder struct {
	ID         int
	CustomerID string
}

type NLineItem struct {
	OrderID    int
	SKU        string
	Name       string
	Quantity   int
	UnitCents  int
}

// NormalizedRepo is the port for the normalized model.
type NormalizedRepo interface {
	// OrderView reassembles an order: needs 3 table lookups + a join.
	OrderView(orderID int) (string, error)
}

// normalized is the in-memory adapter.
type normalized struct {
	customers []NCustomer
	orders    []NOrder
	items     []NLineItem
}

// NewNormalized seeds a tiny normalized store with fictional data.
func NewNormalized() NormalizedRepo {
	return &normalized{
		customers: []NCustomer{{ID: "cust-100", Name: "Alba's Bakery", City: "Stamford"}},
		orders:    []NOrder{{ID: 1, CustomerID: "cust-100"}},
		items: []NLineItem{
			{OrderID: 1, SKU: "flour-25", Name: "Flour 25lb", Quantity: 4, UnitCents: 1899},
			{OrderID: 1, SKU: "yeast-2", Name: "Yeast 2lb", Quantity: 2, UnitCents: 749},
		},
	}
}

func (n *normalized) OrderView(orderID int) (string, error) {
	var order *NOrder
	for i := range n.orders {
		if n.orders[i].ID == orderID {
			order = &n.orders[i]
		}
	}
	if order == nil {
		return "", fmt.Errorf("order %d not found", orderID)
	}
	var cust *NCustomer
	for i := range n.customers {
		if n.customers[i].ID == order.CustomerID {
			cust = &n.customers[i]
		}
	}
	var items []LineItem
	for _, it := range n.items {
		if it.OrderID == orderID {
			items = append(items, LineItem{SKU: it.SKU, Name: it.Name, Quantity: it.Quantity, UnitCents: it.UnitCents})
		}
	}
	return fmt.Sprintf("order #%d — %s (%s): %d line items (assembled from 3 tables: customers, orders, line_items)",
		order.ID, cust.Name, cust.City, len(items)), nil
}

// ---------------------------------------------------------------------------
// Model B — one denormalized document per order (NoSQL style)
// ---------------------------------------------------------------------------

// OrderDoc is a single self-contained document: everything a UI needs to
// render the order lives here, customer details embedded.
type OrderDoc struct {
	OrderID  int    `json:"order_id"`
	Customer struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		City string `json:"city"`
	} `json:"customer"`
	Items []LineItem `json:"items"`
}

// DocumentRepo is the port for the document model.
type DocumentRepo interface {
	// OrderDoc fetches one document by key: a single lookup.
	OrderDoc(orderID int) (string, error)
	OrderDocJSON(orderID int) (string, error)
}

type document struct {
	docs map[int]OrderDoc
}

// NewDocument seeds a document store with the same fictional order.
func NewDocument() DocumentRepo {
	d := OrderDoc{OrderID: 1}
	d.Customer.ID, d.Customer.Name, d.Customer.City = "cust-100", "Alba's Bakery", "Stamford"
	d.Items = []LineItem{
		{SKU: "flour-25", Name: "Flour 25lb", Quantity: 4, UnitCents: 1899},
		{SKU: "yeast-2", Name: "Yeast 2lb", Quantity: 2, UnitCents: 749},
	}
	return &document{docs: map[int]OrderDoc{1: d}}
}

func (d *document) OrderDoc(orderID int) (string, error) {
	doc, ok := d.docs[orderID]
	if !ok {
		return "", fmt.Errorf("order %d not found", orderID)
	}
	return fmt.Sprintf("order #%d — %s (%s): %d line items (one document, single key lookup, no joins)",
		doc.OrderID, doc.Customer.Name, doc.Customer.City, len(doc.Items)), nil
}

func (d *document) OrderDocJSON(orderID int) (string, error) {
	doc, ok := d.docs[orderID]
	if !ok {
		return "", fmt.Errorf("order %d not found", orderID)
	}
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}
