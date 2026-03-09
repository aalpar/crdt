package dotcontext

import (
	"slices"
	"sort"
)

// CausalContext is a compressed representation of a set of observed dots.
// It combines a version vector (replica → max contiguous sequence number)
// with per-replica sorted slices of outlier sequence numbers above the
// contiguous frontier.
type CausalContext struct {
	vv       map[ReplicaID]uint64
	outliers map[ReplicaID][]uint64
}

// New returns an empty CausalContext.
func New() *CausalContext {
	q := &CausalContext{
		vv:       make(map[ReplicaID]uint64),
		outliers: make(map[ReplicaID][]uint64),
	}
	return q
}

// Has reports whether the dot has been observed.
func (p *CausalContext) Has(d Dot) bool {
	if d.Seq <= p.vv[d.ID] {
		return true
	}
	_, found := slices.BinarySearch(p.outliers[d.ID], d.Seq)
	return found
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
	if seqs := p.outliers[id]; len(seqs) > 0 && seqs[len(seqs)-1] > max {
		max = seqs[len(seqs)-1]
	}
	return max
}

// Add records a dot as observed. If it extends the contiguous frontier,
// it goes directly into the version vector. If it is above the frontier
// but non-contiguous, it becomes an outlier. Dots already covered by
// the version vector are ignored.
func (p *CausalContext) Add(d Dot) {
	if d.Seq == p.vv[d.ID]+1 {
		p.vv[d.ID] = d.Seq
	} else if d.Seq > p.vv[d.ID] {
		seqs := p.outliers[d.ID]
		i, found := slices.BinarySearch(seqs, d.Seq)
		if !found {
			p.outliers[d.ID] = slices.Insert(seqs, i, d.Seq)
		}
	}
}

// Merge incorporates all dots from other into p.
func (p *CausalContext) Merge(other *CausalContext) {
	for id, seq := range other.vv {
		if seq > p.vv[id] {
			p.vv[id] = seq
		}
	}
	for id, otherSeqs := range other.outliers {
		frontier := p.vv[id]
		local := p.outliers[id]
		if len(otherSeqs) == 0 {
			continue
		}
		// Sorted merge: both slices are sorted, produce a merged sorted
		// slice in O(len(local) + len(otherSeqs)) instead of O(m×n).
		merged := make([]uint64, 0, len(local)+len(otherSeqs))
		i, j := 0, 0
		for i < len(local) && j < len(otherSeqs) {
			lv, rv := local[i], otherSeqs[j]
			if lv < rv {
				merged = append(merged, lv)
				i++
			} else if lv > rv {
				if rv > frontier {
					merged = append(merged, rv)
				}
				j++
			} else {
				// duplicate — take one copy
				merged = append(merged, lv)
				i++
				j++
			}
		}
		merged = append(merged, local[i:]...)
		for ; j < len(otherSeqs); j++ {
			if otherSeqs[j] > frontier {
				merged = append(merged, otherSeqs[j])
			}
		}
		if len(merged) == 0 {
			delete(p.outliers, id)
		} else {
			p.outliers[id] = merged
		}
	}
}

// Compact cleans up the outlier set: outliers at or below the version
// vector frontier are removed as redundant, and outliers contiguous
// with the frontier are promoted into the version vector. Call after
// batching multiple Add or Merge operations.
func (p *CausalContext) Compact() {
	for id, seqs := range p.outliers {
		frontier := p.vv[id]
		// Skip outliers at or below the frontier (redundant).
		start := sort.Search(len(seqs), func(i int) bool { return seqs[i] > frontier })
		// Promote contiguous outliers into the version vector.
		for start < len(seqs) && seqs[start] == frontier+1 {
			frontier++
			start++
		}
		if frontier > p.vv[id] {
			p.vv[id] = frontier
		}
		if start >= len(seqs) {
			delete(p.outliers, id)
		} else {
			p.outliers[id] = seqs[start:]
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
	for id := range p.outliers {
		seen[id] = struct{}{}
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
		outliers: make(map[ReplicaID][]uint64, len(p.outliers)),
	}
	for id, seq := range p.vv {
		cc.vv[id] = seq
	}
	for id, seqs := range p.outliers {
		cc.outliers[id] = slices.Clone(seqs)
	}
	return cc
}

// Missing returns the dots that remote has observed but p has not,
// grouped by replica and compressed into sorted, non-overlapping,
// non-adjacent SeqRange slices.
func (p *CausalContext) Missing(remote *CausalContext) map[ReplicaID][]SeqRange {
	q := make(map[ReplicaID][]SeqRange)

	// Phase 1: version vector comparison with hole-punching.
	for id, remoteSeq := range remote.vv {
		localSeq := p.vv[id]
		if remoteSeq <= localSeq {
			continue
		}
		lo := localSeq + 1

		// Local outliers for this replica within [lo, remoteSeq].
		// Already sorted — no sort needed.
		localOutliers := p.outliers[id]
		startIdx := sort.Search(len(localOutliers), func(i int) bool { return localOutliers[i] >= lo })
		endIdx := sort.Search(len(localOutliers), func(i int) bool { return localOutliers[i] > remoteSeq })
		holes := localOutliers[startIdx:endIdx]

		if len(holes) == 0 {
			q[id] = append(q[id], SeqRange{Lo: lo, Hi: remoteSeq})
			continue
		}

		cursor := lo
		for _, h := range holes {
			if h > cursor {
				q[id] = append(q[id], SeqRange{Lo: cursor, Hi: h - 1})
			}
			cursor = h + 1
		}
		if cursor <= remoteSeq {
			q[id] = append(q[id], SeqRange{Lo: cursor, Hi: remoteSeq})
		}
	}

	// Phase 2: remote outliers not observed locally.
	for id, seqs := range remote.outliers {
		for _, seq := range seqs {
			if !p.Has(Dot{ID: id, Seq: seq}) {
				q[id] = append(q[id], SeqRange{Lo: seq, Hi: seq})
			}
		}
	}

	// Phase 3: merge ranges per replica (outlier singletons may be adjacent to VV ranges).
	for id, ranges := range q {
		q[id] = mergeRanges(ranges)
	}

	if len(q) == 0 {
		return nil
	}
	return q
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
