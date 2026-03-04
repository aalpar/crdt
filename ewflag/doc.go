// Package ewflag implements an enable-wins flag (EWFlag), a delta-state
// CRDT where concurrent enable and disable resolves in favor of enable.
//
// The implementation composes a bare DotSet from the dotcontext package.
// A non-empty dot set means the flag is enabled; empty means disabled.
// Enable adds a new dot; disable records all current dots as observed
// and clears the store. A concurrent enable introduces a dot unseen by
// the disable's context, so it survives the join — hence "enable-wins."
//
// Mutators return deltas suitable for replication. Merge incorporates
// a delta or full state from another replica.
package ewflag
