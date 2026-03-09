// Package dwflag implements a disable-wins flag (DWFlag), a delta-state
// CRDT where concurrent enable and disable resolves in favor of disable.
//
// The implementation composes a bare DotSet from the dotcontext package.
// An empty dot set means the flag is enabled; non-empty means disabled.
// Disable adds a new dot; enable records all current dots as observed
// and clears the store. A concurrent disable introduces a dot unseen by
// the enable's context, so it survives the join — hence "disable-wins."
//
// Mutators return deltas suitable for replication. Merge incorporates
// a delta or full state from another replica.
package dwflag
