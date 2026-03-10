package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/aalpar/crdt/awset"
	"github.com/aalpar/crdt/ewflag"
	"github.com/aalpar/crdt/lwwregister"
	"github.com/aalpar/crdt/pncounter"
	"github.com/aalpar/crdt/rga"
)

func main() {
	demoAWSet()
	demoPNCounter()
	demoEWFlag()
	demoLWWRegister()
	demoRGA()
}

// demoAWSet shows add-wins conflict resolution.
// Two replicas diverge: one adds "milk", the other removes it.
// After syncing, "milk" survives — add wins over concurrent remove.
func demoAWSet() {
	printHeader("AWSet: Add-Wins Conflict Resolution")

	alice := awset.New[string]("alice")
	bob := awset.New[string]("bob")

	// Both start with the same state: {eggs, milk}
	d1 := alice.Add("eggs")
	d2 := alice.Add("milk")
	bob.Merge(d1)
	bob.Merge(d2)

	printState("initial (synced)", "alice", setOf(alice), "bob", setOf(bob))

	// Network partition: alice re-adds "milk" (new dot), bob removes "milk"
	fmt.Println("  --- network partition ---")
	fmt.Println("  alice: Add(\"milk\")    (re-add creates a new dot)")
	fmt.Println("  bob:   Remove(\"milk\")")
	aliceDelta := alice.Add("milk")
	bobDelta := bob.Remove("milk")

	printState("diverged", "alice", setOf(alice), "bob", setOf(bob))

	// Sync: exchange deltas
	fmt.Println("  --- sync ---")
	alice.Merge(bobDelta)
	bob.Merge(aliceDelta)

	printState("converged", "alice", setOf(alice), "bob", setOf(bob))
	fmt.Println("  milk survives: alice's new dot was unobserved by bob's remove → add wins")
	fmt.Println()
}

// demoPNCounter shows concurrent increments converging.
// Three replicas increment independently, then merge to the correct total.
func demoPNCounter() {
	printHeader("PNCounter: Concurrent Increments")

	alice := pncounter.New("alice")
	bob := pncounter.New("bob")
	carol := pncounter.New("carol")

	// Each replica increments independently
	fmt.Println("  alice: +5, bob: +3, carol: -2")
	dAlice := alice.Increment(5)
	dBob := bob.Increment(3)
	dCarol := carol.Increment(-2)

	fmt.Printf("  before sync: alice=%d  bob=%d  carol=%d\n",
		alice.Value(), bob.Value(), carol.Value())

	// Full sync: everyone merges everyone else's delta
	alice.Merge(dBob)
	alice.Merge(dCarol)
	bob.Merge(dAlice)
	bob.Merge(dCarol)
	carol.Merge(dAlice)
	carol.Merge(dBob)

	fmt.Printf("  after sync:  alice=%d  bob=%d  carol=%d\n",
		alice.Value(), bob.Value(), carol.Value())
	fmt.Println("  all converge to 5 + 3 + (-2) = 6")
	fmt.Println()
}

// demoEWFlag shows enable-wins conflict resolution.
// One replica enables, the other disables — enable wins.
func demoEWFlag() {
	printHeader("EWFlag: Enable-Wins Conflict Resolution")

	alice := ewflag.New("alice")
	bob := ewflag.New("bob")

	// Start enabled on both
	d := alice.Enable()
	bob.Merge(d)

	fmt.Printf("  initial (synced): alice=%v  bob=%v\n", alice.Value(), bob.Value())

	// Partition: alice enables again, bob disables
	fmt.Println("  --- network partition ---")
	fmt.Println("  alice: Enable()")
	fmt.Println("  bob:   Disable()")
	aliceDelta := alice.Enable()
	bobDelta := bob.Disable()

	fmt.Printf("  diverged: alice=%v  bob=%v\n", alice.Value(), bob.Value())

	// Sync
	fmt.Println("  --- sync ---")
	alice.Merge(bobDelta)
	bob.Merge(aliceDelta)

	fmt.Printf("  converged: alice=%v  bob=%v\n", alice.Value(), bob.Value())
	fmt.Println("  concurrent Enable + Disable → enable wins")
	fmt.Println()
}

// demoLWWRegister shows last-writer-wins by timestamp.
// Two replicas write concurrently with different timestamps.
func demoLWWRegister() {
	printHeader("LWWRegister: Last-Writer-Wins by Timestamp")

	alice := lwwregister.New[string]("alice")
	bob := lwwregister.New[string]("bob")

	// Initial sync
	d := alice.Set("v1", 100)
	bob.Merge(d)
	printRegister("initial (synced)", "alice", alice, "bob", bob)

	// Partition: concurrent writes with different timestamps
	fmt.Println("  --- network partition ---")
	fmt.Println("  alice: Set(\"alice-wins\", ts=300)")
	fmt.Println("  bob:   Set(\"bob-loses\", ts=200)")
	aliceDelta := alice.Set("alice-wins", 300)
	bobDelta := bob.Set("bob-loses", 200)

	printRegister("diverged", "alice", alice, "bob", bob)

	// Sync
	fmt.Println("  --- sync ---")
	alice.Merge(bobDelta)
	bob.Merge(aliceDelta)

	printRegister("converged", "alice", alice, "bob", bob)
	fmt.Println("  both resolve to \"alice-wins\" (ts=300 > ts=200)")
	fmt.Println()
}

// demoRGA shows concurrent inserts into a sequence.
// Two replicas insert at the same position — both elements survive,
// ordered deterministically by dot.
func demoRGA() {
	printHeader("RGA: Concurrent Sequence Inserts")

	alice := rga.New[string]("alice")
	bob := rga.New[string]("bob")

	// Build initial sequence: [H, E, L, O]
	d1 := alice.InsertAt(0, "H")
	d2 := alice.InsertAt(1, "E")
	d3 := alice.InsertAt(2, "L")
	d4 := alice.InsertAt(3, "O")
	bob.Merge(d1)
	bob.Merge(d2)
	bob.Merge(d3)
	bob.Merge(d4)

	printRGA("initial (synced)", "alice", alice, "bob", bob)

	// Partition: both insert after position 1 ("E")
	fmt.Println("  --- network partition ---")
	fmt.Println("  alice: InsertAt(2, \"X\")  (after E)")
	fmt.Println("  bob:   InsertAt(2, \"Y\")  (after E)")
	aliceDelta := alice.InsertAt(2, "X")
	bobDelta := bob.InsertAt(2, "Y")

	printRGA("diverged", "alice", alice, "bob", bob)

	// Sync
	fmt.Println("  --- sync ---")
	alice.Merge(bobDelta)
	bob.Merge(aliceDelta)

	printRGA("converged", "alice", alice, "bob", bob)
	fmt.Println("  both inserts preserved, ordered deterministically by dot")

	// Demonstrate delete
	fmt.Println()
	fmt.Println("  alice: DeleteAt(2)  (removes first concurrent insert)")
	dDel := alice.DeleteAt(2)
	bob.Merge(dDel)

	printRGA("after delete", "alice", alice, "bob", bob)
	fmt.Println()
}

// --- helpers ---

func printHeader(title string) {
	bar := strings.Repeat("─", len(title)+2)
	fmt.Printf("┌%s┐\n", bar)
	fmt.Printf("│ %s │\n", title)
	fmt.Printf("└%s┘\n", bar)
}

func setOf(s *awset.AWSet[string]) string {
	elems := s.Elements()
	sort.Strings(elems)
	return "{" + strings.Join(elems, ", ") + "}"
}

func printState(label, name1, val1, name2, val2 string) {
	fmt.Printf("  %-12s %s=%s  %s=%s\n", label+":", name1, val1, name2, val2)
}

func printRegister(label string, name1 string, r1 *lwwregister.LWWRegister[string], name2 string, r2 *lwwregister.LWWRegister[string]) {
	fmt.Printf("  %-12s %s=%s  %s=%s\n", label+":", name1, regStr(r1), name2, regStr(r2))
}

func regStr(r *lwwregister.LWWRegister[string]) string {
	v, ts, ok := r.Value()
	if !ok {
		return "<empty>"
	}
	return fmt.Sprintf("%q (ts=%d)", v, ts)
}

func printRGA(label string, name1 string, r1 *rga.RGA[string], name2 string, r2 *rga.RGA[string]) {
	fmt.Printf("  %-12s %s=%s  %s=%s\n", label+":", name1, rgaStr(r1), name2, rgaStr(r2))
}

func rgaStr(r *rga.RGA[string]) string {
	elems := r.Elements()
	parts := make([]string, len(elems))
	for i, e := range elems {
		parts[i] = e.Value
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
