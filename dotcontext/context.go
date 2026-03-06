package dotcontext

import "sort"

// CausalContext is a compressed representation of a set of observed dots.
// It combines a version vector (replica → max contiguous sequence number)
// with a set of outlier dots above the contiguous frontier.
type CausalContext struct {
	vv       map[ReplicaID]uint64
	outliers map[Dot]struct{}
}

// New returns an empty CausalContext.
func New() *CausalContext {
	q := &CausalContext{
		vv:       make(map[ReplicaID]uint64),
		outliers: make(map[Dot]struct{}),
	}
	return q
}

// Has reports whether the dot has been observed.
func (p *CausalContext) Has(d Dot) bool {
	if d.Seq <= p.vv[d.ID] {
		return true
	}
	_, ok := p.outliers[d]
	return ok
}

// Next returns the next dot for the given replica and adds it to the context.
func (p *CausalContext) Next(id ReplicaID) Dot {
	seq := p.vv[id] + 1
	p.vv[id] = seq
	return Dot{ID: id, Seq: seq}
}

// Max returns the maximum observed sequence number for a replica.
// This includes outliers.
func (p *CausalContext) Max(id ReplicaID) uint64 {
	max := p.vv[id]
	for d := range p.outliers {
		if d.ID == id && d.Seq > max {
			max = d.Seq
		}
	}
	return max
}

// Add records a dot as observed. If it extends the contiguous range,
// it goes directly into the version vector. Otherwise it becomes an outlier.
func (p *CausalContext) Add(d Dot) {
	if d.Seq == p.vv[d.ID]+1 {
		p.vv[d.ID] = d.Seq
	} else if d.Seq > p.vv[d.ID] {
		p.outliers[d] = struct{}{}
	}
}

// Merge incorporates all dots from other into p.
func (p *CausalContext) Merge(other *CausalContext) {
	for id, seq := range other.vv {
		if seq > p.vv[id] {
			p.vv[id] = seq
		}
	}
	for d := range other.outliers {
		if !p.Has(d) {
			p.outliers[d] = struct{}{}
		}
	}
}

// Compact promotes outliers into the version vector when they are
// contiguous with the current frontier. Call this after batching
// multiple Add or Merge operations.
func (p *CausalContext) Compact() {
	changed := true
	for changed {
		changed = false
		for d := range p.outliers {
			if d.Seq <= p.vv[d.ID] {
				// Redundant: already covered by version vector.
				delete(p.outliers, d)
				changed = true
			} else if d.Seq == p.vv[d.ID]+1 {
				// Contiguous: promote into version vector.
				p.vv[d.ID] = d.Seq
				delete(p.outliers, d)
				changed = true
			}
		}
	}
}

// ReplicaIDs returns all replica IDs known to this context.
// This includes replicas from both the version vector and outliers.
func (p *CausalContext) ReplicaIDs() []ReplicaID {
	seen := make(map[ReplicaID]struct{}, len(p.vv))
	for id := range p.vv {
		seen[id] = struct{}{}
	}
	for d := range p.outliers {
		seen[d.ID] = struct{}{}
	}
	ids := make([]ReplicaID, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	return ids
}

// Clone returns a deep copy.
func (p *CausalContext) Clone() *CausalContext {
	cc := &CausalContext{
		vv:       make(map[ReplicaID]uint64, len(p.vv)),
		outliers: make(map[Dot]struct{}, len(p.outliers)),
	}
	for id, seq := range p.vv {
		cc.vv[id] = seq
	}
	for d := range p.outliers {
		cc.outliers[d] = struct{}{}
	}
	return cc
}

// mergeRanges sorts ranges by Lo and merges overlapping or adjacent ranges.
func mergeRanges(ranges []SeqRange) []SeqRange {
	if len(ranges) <= 1 {
		return ranges
	}
	sort.Slice(ranges, func(i, j int) bool {
		return ranges[i].Lo < ranges[j].Lo
	})
	q := ranges[:1]
	for _, r := range ranges[1:] {
		last := &q[len(q)-1]
		if r.Lo <= last.Hi+1 {
			if r.Hi > last.Hi {
				last.Hi = r.Hi
			}
		} else {
			q = append(q, r)
		}
	}
	return q
}
