package dotcontext

import "fmt"

// Dot is a unique event identifier: a (replica, sequence number) pair.
// Each replica generates dots with monotonically increasing sequence numbers.
type Dot struct {
	ID  string // replica identifier
	Seq uint64 // monotonically increasing per replica
}

// String returns "id:seq".
func (d Dot) String() string {
	return fmt.Sprintf("%s:%d", d.ID, d.Seq)
}
