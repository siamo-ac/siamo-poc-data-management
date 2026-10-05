// Package shard demonstrates shard-key routing: deciding which physical
// shard a record lives on from its key. Two strategies are shown:
//   - ModuloRouter: hash(key) % N. Dead simple, but adding a shard
//     reshuffles almost every key.
//   - HashRing: consistent hashing. Adding a shard moves only the keys
//     that land on the new node.
//
// The Router interface is the port; ModuloRouter and HashRing are adapters.
package shard

import (
	"fmt"
	"hash/fnv"
	"sort"
	"strconv"
)

// Router is the port: map a key to a shard name.
type Router interface {
	// ShardFor returns the shard responsible for key.
	ShardFor(key string) string
	// Shards returns the current shard names.
	Shards() []string
}

func hash64(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return h.Sum64()
}

// ---------------------------------------------------------------------------
// ModuloRouter — hash(key) % len(shards)
// ---------------------------------------------------------------------------

// ModuloRouter is the simplest possible sharder.
type ModuloRouter struct {
	shards []string
}

// NewModuloRouter builds a modulo router over n shards named shard-0..n-1.
func NewModuloRouter(n int) *ModuloRouter {
	shards := make([]string, n)
	for i := range shards {
		shards[i] = fmt.Sprintf("shard-%d", i)
	}
	return &ModuloRouter{shards: shards}
}

func (r *ModuloRouter) ShardFor(key string) string {
	return r.shards[hash64(key)%uint64(len(r.shards))]
}

func (r *ModuloRouter) Shards() []string { return append([]string{}, r.shards...) }

// AddShard returns a NEW router with one more shard (modulo rehashes).
func (r *ModuloRouter) AddShard() *ModuloRouter {
	return NewModuloRouter(len(r.shards) + 1)
}

// ---------------------------------------------------------------------------
// HashRing — consistent hashing with virtual nodes
// ---------------------------------------------------------------------------

// HashRing implements consistent hashing: each physical shard owns several
// virtual nodes (vnodes) spread around a hash ring. A key lands on the
// first vnode clockwise from its hash.
type HashRing struct {
	vnodes   int
	ring     []uint64            // sorted vnode hashes
	owners   map[uint64]string   // vnode hash -> shard name
	shards   []string
}

// NewHashRing builds a consistent-hash ring over n shards with vnodes
// virtual nodes each.
func NewHashRing(n, vnodes int) *HashRing {
	r := &HashRing{
		vnodes: vnodes,
		owners: map[uint64]string{},
	}
	for i := 0; i < n; i++ {
		r.AddShard(fmt.Sprintf("shard-%d", i))
	}
	return r
}

// AddShard adds a physical shard; only keys mapping to its vnodes move.
func (r *HashRing) AddShard(name string) {
	r.shards = append(r.shards, name)
	for v := 0; v < r.vnodes; v++ {
		h := hash64(name + "#" + strconv.Itoa(v))
		r.ring = append(r.ring, h)
		r.owners[h] = name
	}
	sort.Slice(r.ring, func(i, j int) bool { return r.ring[i] < r.ring[j] })
}

func (r *HashRing) ShardFor(key string) string {
	h := hash64(key)
	// first vnode at or after h, wrapping around the ring
	i := sort.Search(len(r.ring), func(i int) bool { return r.ring[i] >= h })
	if i == len(r.ring) {
		i = 0
	}
	return r.owners[r.ring[i]]
}

func (r *HashRing) Shards() []string { return append([]string{}, r.shards...) }

// ---------------------------------------------------------------------------
// In-memory sharded store
// ---------------------------------------------------------------------------

// Record is one fictional customer record. Keys are customer IDs.
type Record struct {
	Key  string
	Name string
	City string
}

// Store is a sharded in-memory key/value store driven by a Router.
type Store struct {
	router Router
	shards map[string]map[string]Record
}

// NewStore creates a store whose records are placed by router.
func NewStore(router Router) *Store {
	s := &Store{router: router, shards: map[string]map[string]Record{}}
	for _, name := range router.Shards() {
		s.shards[name] = map[string]Record{}
	}
	return s
}

// Put routes the record to its shard.
func (s *Store) Put(rec Record) string {
	shard := s.router.ShardFor(rec.Key)
	if _, ok := s.shards[shard]; !ok {
		s.shards[shard] = map[string]Record{}
	}
	s.shards[shard][rec.Key] = rec
	return shard
}

// Get fetches a record by key, touching only its shard.
func (s *Store) Get(key string) (Record, string, bool) {
	shard := s.router.ShardFor(key)
	rec, ok := s.shards[shard][key]
	return rec, shard, ok
}

// Distribution reports how many records each shard holds.
func (s *Store) Distribution() map[string]int {
	d := map[string]int{}
	for name, m := range s.shards {
		d[name] = len(m)
	}
	return d
}
