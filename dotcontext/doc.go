// Package dotcontext implements causal contexts and dot stores for
// delta-state CRDTs, following Almeida et al. 2018 ("Delta State
// Replicated Data Types").
//
// The type hierarchy:
//
//	Dot                          unique event identifier (replica, seq)
//	CausalContext                compressed set of observed dots
//	DotStore (interface)         anything that can enumerate its dots
//	  DotSet                     set of dots — P(I × N)
//	  DotFun[V Lattice[V]]      dots mapped to lattice values
//	  DotMap[K, V DotStore]     keys mapped to nested dot stores
//	Causal[T DotStore]           dot store + causal context
//
// Join functions merge two Causal values, implementing the
// semilattice algebra (idempotent, commutative, associative).
// In the formulas below, sᵢ is the dot store of replica i and
// cᵢ is its causal context (the set of dots it has observed):
//
//	JoinDotSet   — (s₁ ∩ s₂) ∪ (s₁ \ c₂) ∪ (s₂ \ c₁)
//	JoinDotFun   — lattice join per dot
//	JoinDotMap   — recursive join with caller-supplied nested join
package dotcontext
