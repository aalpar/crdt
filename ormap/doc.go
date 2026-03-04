// Package ormap implements an observed-remove map (ORMap), a
// delta-state CRDT that maps keys to nested CRDT values of any
// DotStore type.
//
// The implementation composes DotMap[K, V] from the dotcontext
// package. A key exists when its value has active dots; removing a
// key records its dots in the causal context, and concurrent add
// and remove resolves in favor of add (add-wins semantics).
//
// The ORMap is a building block — it handles key-level semantics
// and recursive merge, while the caller provides the nested join
// function and empty-value constructor for the value type V.
//
// Apply mutates the value at a key and returns a delta for
// replication. Remove deletes a key. Merge incorporates a delta
// or full state from another replica.
package ormap
