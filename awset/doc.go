// Package awset implements an add-wins observed-remove set (AWSet),
// a delta-state CRDT where concurrent add and remove of the same
// element resolves in favor of add.
//
// The implementation composes DotMap[E, *DotSet] from the dotcontext
// package. Each element maps to a set of dots tracking which replicas
// have added it. The causal context tracks what has been observed,
// enabling correct remove semantics: a remove only affects dots known
// at the time of the operation, so a concurrent add (with a new,
// unseen dot) survives.
//
// Mutators return deltas suitable for replication. Merge incorporates
// a delta or full state from another replica.
package awset
