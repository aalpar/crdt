package dotcontext

// DotSet is a set of dots: P(I × N) in the paper's notation.
// Used by awset to track which replicas contributed to a value's presence.
type DotSet struct {
	dots map[Dot]struct{}
}

// NewDotSet returns an empty DotSet.
func NewDotSet() *DotSet {
	return &DotSet{dots: make(map[Dot]struct{})}
}

// Add inserts a dot into the set.
func (s *DotSet) Add(d Dot) {
	s.dots[d] = struct{}{}
}

// Remove deletes a dot from the set.
func (s *DotSet) Remove(d Dot) {
	delete(s.dots, d)
}

// Has reports whether the dot is in the set.
func (s *DotSet) Has(d Dot) bool {
	_, ok := s.dots[d]
	return ok
}

// Len returns the number of dots.
func (s *DotSet) Len() int {
	return len(s.dots)
}

// Range calls f for each dot. If f returns false, iteration stops.
func (s *DotSet) Range(f func(Dot) bool) {
	for d := range s.dots {
		if !f(d) {
			return
		}
	}
}

// Dots returns the set itself (DotStore implementation).
func (s *DotSet) Dots() *DotSet {
	return s.Clone()
}

// Clone returns a deep copy.
func (s *DotSet) Clone() *DotSet {
	ds := &DotSet{dots: make(map[Dot]struct{}, len(s.dots))}
	for d := range s.dots {
		ds.dots[d] = struct{}{}
	}
	return ds
}
