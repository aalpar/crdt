package rga

import (
	"io"
	"sort"

	"github.com/aalpar/crdt/dotcontext"
)

// Node is a sequence element stored in a DotFun. It records the
// predecessor dot (After), a tombstone flag, and the element value.
// Node satisfies dotcontext.Lattice: join merges the Deleted flag
// (delete wins — once deleted, always deleted).
type Node[E comparable] struct {
	Value   E
	After   dotcontext.Dot // zero Dot = head
	Deleted bool
}

// Join merges two nodes. The Deleted flag is OR'd: if either side
// has observed a delete, the result is deleted.
func (n Node[E]) Join(other Node[E]) Node[E] {
	return Node[E]{
		Value:   n.Value,
		After:   n.After,
		Deleted: n.Deleted || other.Deleted,
	}
}

// Element is a visible (non-tombstoned) entry returned by queries.
type Element[E comparable] struct {
	ID    dotcontext.Dot
	Value E
}

// RGA is a Replicated Growable Array. It composes a
// DotFun[Node[E]] with a causal context to provide an ordered
// sequence with insert and delete operations.
type RGA[E comparable] struct {
	id    dotcontext.ReplicaID
	state dotcontext.Causal[*dotcontext.DotFun[Node[E]]]
}

// New creates an empty RGA for the given replica.
func New[E comparable](replicaID dotcontext.ReplicaID) *RGA[E] {
	return &RGA[E]{
		id: replicaID,
		state: dotcontext.Causal[*dotcontext.DotFun[Node[E]]]{
			Store:   dotcontext.NewDotFun[Node[E]](),
			Context: dotcontext.New(),
		},
	}
}

// InsertAfter inserts value after the given dot and returns a delta
// for replication. A zero dot means insert at the head of the sequence.
func (r *RGA[E]) InsertAfter(after dotcontext.Dot, value E) *RGA[E] {
	d := r.state.Context.Next(r.id)
	node := Node[E]{Value: value, After: after}
	r.state.Store.Set(d, node)

	deltaStore := dotcontext.NewDotFun[Node[E]]()
	deltaStore.Set(d, node)

	deltaCtx := dotcontext.New()
	deltaCtx.Add(d)

	return &RGA[E]{
		state: dotcontext.Causal[*dotcontext.DotFun[Node[E]]]{
			Store:   deltaStore,
			Context: deltaCtx,
		},
	}
}

// Delete tombstones the node at the given dot and returns a delta.
// Returns an empty delta if the dot is missing or already deleted.
func (r *RGA[E]) Delete(at dotcontext.Dot) *RGA[E] {
	node, ok := r.state.Store.Get(at)
	if !ok || node.Deleted {
		return r.emptyDelta()
	}

	node.Deleted = true
	r.state.Store.Set(at, node)

	deltaStore := dotcontext.NewDotFun[Node[E]]()
	deltaStore.Set(at, node)

	deltaCtx := dotcontext.New()
	deltaCtx.Add(at)

	return &RGA[E]{
		state: dotcontext.Causal[*dotcontext.DotFun[Node[E]]]{
			Store:   deltaStore,
			Context: deltaCtx,
		},
	}
}

// InsertAt inserts value at the given visible index. Index 0 inserts
// at the head. Returns an empty delta if index is out of bounds
// (except index == Len(), which appends).
func (r *RGA[E]) InsertAt(index int, value E) *RGA[E] {
	if index < 0 || index > r.Len() {
		return r.emptyDelta()
	}
	if index == 0 {
		return r.InsertAfter(dotcontext.Dot{}, value)
	}
	elem, ok := r.At(index - 1)
	if !ok {
		return r.emptyDelta()
	}
	return r.InsertAfter(elem.ID, value)
}

// DeleteAt deletes the element at the given visible index.
// Returns an empty delta if out of bounds.
func (r *RGA[E]) DeleteAt(index int) *RGA[E] {
	elem, ok := r.At(index)
	if !ok {
		return r.emptyDelta()
	}
	return r.Delete(elem.ID)
}

// Elements returns all visible (non-tombstoned) elements in order.
func (r *RGA[E]) Elements() []Element[E] {
	order := r.linearize()
	var result []Element[E]
	for _, d := range order {
		node, _ := r.state.Store.Get(d)
		if !node.Deleted {
			result = append(result, Element[E]{ID: d, Value: node.Value})
		}
	}
	return result
}

// At returns the visible element at the given index. Returns false
// if the index is out of bounds.
func (r *RGA[E]) At(index int) (Element[E], bool) {
	if index < 0 {
		return Element[E]{}, false
	}
	elems := r.Elements()
	if index >= len(elems) {
		return Element[E]{}, false
	}
	return elems[index], true
}

// Len returns the count of visible (non-tombstoned) elements.
func (r *RGA[E]) Len() int {
	count := 0
	r.state.Store.Range(func(_ dotcontext.Dot, n Node[E]) bool {
		if !n.Deleted {
			count++
		}
		return true
	})
	return count
}

// Merge incorporates a delta or full state from another RGA.
func (r *RGA[E]) Merge(other *RGA[E]) {
	dotcontext.MergeDotFun(&r.state, other.state)
}

// State returns the RGA's internal Causal state for serialization.
func (r *RGA[E]) State() dotcontext.Causal[*dotcontext.DotFun[Node[E]]] {
	return r.state
}

// FromCausal constructs an RGA from a decoded Causal value.
// Used to reconstruct deltas from the wire for merging.
func FromCausal[E comparable](state dotcontext.Causal[*dotcontext.DotFun[Node[E]]]) *RGA[E] {
	return &RGA[E]{state: state}
}

// linearize returns all dots in sequence order by DFS from the head.
// Children of each parent are sorted: higher Seq first, then higher
// replica ID first — so newer inserts at the same position appear left.
func (r *RGA[E]) linearize() []dotcontext.Dot {
	// Build children map: parent → []child dots.
	children := make(map[dotcontext.Dot][]dotcontext.Dot)
	r.state.Store.Range(func(d dotcontext.Dot, n Node[E]) bool {
		children[n.After] = append(children[n.After], d)
		return true
	})

	// Sort each sibling list: Seq descending, then ID descending.
	// This puts the highest-priority (newest) insert first in DFS,
	// which means it appears leftmost.
	for _, siblings := range children {
		sort.Slice(siblings, func(i, j int) bool {
			if siblings[i].Seq != siblings[j].Seq {
				return siblings[i].Seq > siblings[j].Seq
			}
			return siblings[i].ID > siblings[j].ID
		})
	}

	// DFS from head (zero Dot).
	var result []dotcontext.Dot
	var dfs func(parent dotcontext.Dot)
	dfs = func(parent dotcontext.Dot) {
		for _, child := range children[parent] {
			result = append(result, child)
			dfs(child)
		}
	}
	dfs(dotcontext.Dot{})

	return result
}

// NodeCodec encodes a Node[E] as [V: value] [Dot: after] [byte: deleted].
type NodeCodec[E comparable] struct {
	ValueCodec dotcontext.Codec[E]
}

func (c NodeCodec[E]) Encode(w io.Writer, n Node[E]) error {
	if err := c.ValueCodec.Encode(w, n.Value); err != nil {
		return err
	}
	if err := (dotcontext.DotCodec{}).Encode(w, n.After); err != nil {
		return err
	}
	var b [1]byte
	if n.Deleted {
		b[0] = 1
	}
	_, err := w.Write(b[:])
	return err
}

func (c NodeCodec[E]) Decode(r io.Reader) (Node[E], error) {
	val, err := c.ValueCodec.Decode(r)
	if err != nil {
		return Node[E]{}, err
	}
	after, err := (dotcontext.DotCodec{}).Decode(r)
	if err != nil {
		return Node[E]{}, err
	}
	var b [1]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return Node[E]{}, err
	}
	return Node[E]{Value: val, After: after, Deleted: b[0] != 0}, nil
}

func (r *RGA[E]) emptyDelta() *RGA[E] {
	return &RGA[E]{
		state: dotcontext.Causal[*dotcontext.DotFun[Node[E]]]{
			Store:   dotcontext.NewDotFun[Node[E]](),
			Context: dotcontext.New(),
		},
	}
}
