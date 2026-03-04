package dotcontext

// Causal pairs a dot store with its causal context. This is the unit
// of replication: a delta or full state is always a Causal value.
// The join of two Causal values produces the merged state.
type Causal[T DotStore] struct {
	Store   T
	Context *CausalContext
}
