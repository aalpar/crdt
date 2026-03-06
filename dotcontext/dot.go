package dotcontext

import "fmt"

// ReplicaID identifies a replica in the system.
type ReplicaID string

// Dot is a unique event identifier: a (replica, sequence number) pair.
// Each replica generates dots with monotonically increasing sequence numbers.
type Dot struct {
	ID  ReplicaID // replica identifier
	Seq uint64    // monotonically increasing per replica
}

// String returns "id:seq".
func (p Dot) String() string {
	return fmt.Sprintf("%s:%d", p.ID, p.Seq)
}

// SeqRange is an inclusive range of sequence numbers [Lo, Hi].
type SeqRange struct {
	Lo, Hi uint64
}

// String returns "[Lo,Hi]".
func (p SeqRange) String() string {
	return fmt.Sprintf("[%d,%d]", p.Lo, p.Hi)
}
