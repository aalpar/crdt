package dotcontext

// CausalContext is a compressed representation of a set of observed dots.
// It combines a version vector (replica → max contiguous sequence number)
// with a set of outlier dots above the contiguous frontier.
type CausalContext struct {
	vv       map[string]uint64
	outliers map[Dot]struct{}
}

// New returns an empty CausalContext.
func New() *CausalContext {
	return &CausalContext{
		vv:       make(map[string]uint64),
		outliers: make(map[Dot]struct{}),
	}
}

// Has reports whether the dot has been observed.
func (c *CausalContext) Has(d Dot) bool {
	if d.Seq <= c.vv[d.ID] {
		return true
	}
	_, ok := c.outliers[d]
	return ok
}

// Next returns the next dot for the given replica and adds it to the context.
func (c *CausalContext) Next(id string) Dot {
	seq := c.vv[id] + 1
	c.vv[id] = seq
	return Dot{ID: id, Seq: seq}
}

// Max returns the maximum observed sequence number for a replica.
// This includes outliers.
func (c *CausalContext) Max(id string) uint64 {
	max := c.vv[id]
	for d := range c.outliers {
		if d.ID == id && d.Seq > max {
			max = d.Seq
		}
	}
	return max
}

// Add records a dot as observed. If it extends the contiguous range,
// it goes directly into the version vector. Otherwise it becomes an outlier.
func (c *CausalContext) Add(d Dot) {
	if d.Seq == c.vv[d.ID]+1 {
		c.vv[d.ID] = d.Seq
	} else if d.Seq > c.vv[d.ID] {
		c.outliers[d] = struct{}{}
	}
}

// Merge incorporates all dots from other into c.
func (c *CausalContext) Merge(other *CausalContext) {
	for id, seq := range other.vv {
		if seq > c.vv[id] {
			c.vv[id] = seq
		}
	}
	for d := range other.outliers {
		if !c.Has(d) {
			c.outliers[d] = struct{}{}
		}
	}
}

// Compact promotes outliers into the version vector when they are
// contiguous with the current frontier. Call this after batching
// multiple Add or Merge operations.
func (c *CausalContext) Compact() {
	changed := true
	for changed {
		changed = false
		for d := range c.outliers {
			if d.Seq == c.vv[d.ID]+1 {
				c.vv[d.ID] = d.Seq
				delete(c.outliers, d)
				changed = true
			}
		}
	}
}

// Clone returns a deep copy.
func (c *CausalContext) Clone() *CausalContext {
	cc := &CausalContext{
		vv:       make(map[string]uint64, len(c.vv)),
		outliers: make(map[Dot]struct{}, len(c.outliers)),
	}
	for id, seq := range c.vv {
		cc.vv[id] = seq
	}
	for d := range c.outliers {
		cc.outliers[d] = struct{}{}
	}
	return cc
}
