package dotcontext

// DotStore is any structure that can enumerate its active dots.
// DotSet, DotFun, and DotMap all implement this interface.
type DotStore interface {
	Dots() *DotSet
}
