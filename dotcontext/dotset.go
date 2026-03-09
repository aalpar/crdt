package dotcontext

// DotSet is a set of dots: P(I × N) in the paper's notation.
// Used by awset to track which replicas contributed to a value's presence.
type DotSet struct {
	dots map[Dot]struct{}
}

// NewDotSet returns an empty DotSet.
func NewDotSet() *DotSet {
	q := &DotSet{dots: make(map[Dot]struct{})}
	return q
}

// Add inserts a dot into the set.
func (p *DotSet) Add(d Dot) {
	p.dots[d] = struct{}{}
}

// Remove deletes a dot from the set.
func (p *DotSet) Remove(d Dot) {
	delete(p.dots, d)
}

// Has reports whether the dot is in the set.
func (p *DotSet) Has(d Dot) bool {
	_, ok := p.dots[d]
	return ok
}

// Len returns the number of dots.
func (p *DotSet) Len() int {
	return len(p.dots)
}

// Range calls f for each dot. If f returns false, iteration stops.
func (p *DotSet) Range(f func(Dot) bool) {
	for d := range p.dots {
		if !f(d) {
			return
		}
	}
}

// Dots returns a clone of this set (DotStore implementation).
// The caller may mutate the result without affecting the original.
func (p *DotSet) Dots() *DotSet {
	return p.Clone()
}

// Clone returns a deep copy.
func (p *DotSet) Clone() *DotSet {
	ds := &DotSet{dots: make(map[Dot]struct{}, len(p.dots))}
	for d := range p.dots {
		ds.dots[d] = struct{}{}
	}
	return ds
}
