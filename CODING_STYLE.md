# Coding Style Guide

This document describes the coding conventions used throughout this codebase.

Run `go vet` and `golangci-lint run` before committing.

## Functions

Never write single-line functions. Always spread function bodies across multiple lines, even for simple implementations.

### Early Exit From Functions When Possible

**A: Classic Nested If (Less Idiomatic)**
```go
func processDataNested(value int, settings map[string]bool) error {
	if value > 0 {
		if settings != nil {
			if settings["enabled"] {
				fmt.Printf("Processing value: %d\n", value)
				return nil
			} else {
				return fmt.Errorf("settings not enabled")
			}
		} else {
			return fmt.Errorf("settings is nil")
		}
	} else {
		return fmt.Errorf("value must be positive")
	}
}
```

**B: Early Return (Idiomatic Go)**
- General structure:
    -- Check preconditions necessary to eliminate runtime errors, and known failure modes of subsequent function calls.
    -- Attempt to avoid computation in the body by dealing with edge-cases and known trivial cases first.
```go
func mult(a, b int) int {
	if a == 0 || b == 0 {
		return 0
	}
	return a * b
}
```

- This approach uses "guard clauses" at the beginning of the function to handle error conditions immediately and return, which keeps the main logic (the "happy path") flat and easy to read.

```go
func processDataEarlyReturn(value int, settings map[string]bool) error {
	if value <= 0 {
		return fmt.Errorf("value must be positive")
	}
	if settings == nil {
		return fmt.Errorf("settings is nil")
	}
	if !settings["enabled"] {
		return fmt.Errorf("settings not enabled")
	}
	fmt.Printf("Processing value: %d\n", value)
	return nil
}
```

#### Key Advantages of Early Return (B)
- **Readability**: The code reads more linearly, making the primary execution flow (the "happy path") much clearer.
- **Reduced Nesting**: It avoids the "arrow code" or "deep nesting" problem associated with multiple if/else blocks.
- **Maintainability**: It is easier to add or modify a validation check without affecting the indentation or structure of the main logic.
- **Idiomatic Go**: This style is explicitly encouraged in [Effective Go](https://go.dev/doc/effective_go#if).

## Return Values

| Letter | Usage                                                                                                                                                                                          |
|--------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `q`    | Variable name for value that is eventually returned. Only used when returning single value or returning two values, the first value being `q` and the second value being an `error` type |

## Receiver Naming

All method receivers use `p`:

```go
func (p *AWSet[E]) Add(elem E) *AWSet[E] { ... }
func (p *CausalContext) Next(id ReplicaID) Dot { ... }
func (p *DotMap[K, V]) Get(k K) (V, bool) { ... }
```

**Never use:**
- Descriptive names like `this`, `self`, `receiver`
- Type-abbreviations like `s`, `m`, `c`, `f`, `r`
- Multi-letter names like `ctx`, `set`, `map`

## Variable Naming

### Return Values: `q`

All `New*` constructors use `q` for the intermediate variable:

```go
func NewDotSet() *DotSet {
	q := &DotSet{dots: make(map[Dot]struct{})}
	return q
}

func New[E comparable](replicaID ReplicaID) *AWSet[E] {
	q := &AWSet[E]{
		id:    replicaID,
		state: Causal[*DotMap[E, *DotSet]]{...},
	}
	return q
}
```

### Standard Variable Names

| Name | Usage |
|------|-------|
| `i`, `j`, `k` | Loop counters |
| `n` | Count, length, or numeric delta |
| `v` | Temporary value in type switches or range |
| `ok` | Boolean result from map lookups and type assertions |
| `q` | Return value |
| `err` | Error values |
| `o` | "Other" operand in binary operations |
| `id` | Replica identifier parameter |
| `d` | Dot value (in dot-related operations) |
| `fn` | Function parameter (callbacks, iterators) |

```go
// Type assertion pattern
if other, ok := v.(*DotSet); ok { ... }

// Binary operation pattern
func (p *counterValue) Join(o counterValue) counterValue { ... }

// Iterator callback pattern
p.dots.Range(func(d Dot) bool { ... })
```

### Test Variable Names

Table-driven test structs use numbered inputs/outputs:

```go
tcs := []struct {
	in0  *AWSet[string]
	in1  *AWSet[string]
	out  []string
}{
	{in0: New[string]("a"), in1: New[string]("b")},
}
```

## Function Naming

### Prefixes

| Prefix | Meaning | Returns |
|--------|---------|---------|
| `New` | Constructor | `*Type` |
| `Is` | Predicate | `bool` |
| `Set` | Setter | Usually nothing |
| `Get` | Getter | Value |
| `May` | Optional operation (may not perform) | Varies |
| `Must` | Required operation (panics on failure) | Varies |

### Constructor Variants

When multiple constructors exist, use `From` suffix to indicate source:

```go
New(replicaID ReplicaID) *AWSet[E]
NewFromSnapshot(snap Snapshot) *AWSet[E]
```

## Control Flow

### If Statement Assignments

**Never** combine assignment and comparison in a single `if` statement. Always separate them:

```go
// Correct - assignment on separate line
err := doSomething()
if err != nil {
	return err
}

// Correct - multiple assignments separated
result, ok := p.state.Store.Get(elem)
if !ok {
	return nil
}

// Avoid - combined assignment and comparison
if err := doSomething(); err != nil {  // DON'T
	return err
}

// Avoid - combined with map lookup
if v, ok := m[key]; ok {  // DON'T
	// ...
}
```

**Rationale:**
- Improves readability by keeping operations atomic
- Makes debugging easier (can set breakpoints on assignment)
- Variables are available in the outer scope when needed

**Exception:** Short-circuit boolean expressions are acceptable:
```go
if x != nil && x.IsValid() {  // OK - no assignment
	// ...
}
```

## Error Handling

### Error Types

Use standard library errors:

1. **Sentinel errors** - Pre-created constants for common errors:
   ```go
   var (
       ErrInvalidReplica = errors.New("invalid replica ID")
       ErrEmptyDelta     = errors.New("empty delta")
   )
   ```

2. **Wrapped errors** - Adding context to existing errors:
   ```go
   fmt.Errorf("CausalContext.Next: %w", err)
   fmt.Errorf("AWSet.Merge: replica %q: %w", id, err)
   ```

### Error Comparison

**Always** use `errors.Is` for sentinel error checks, never `==`:

```go
// Correct
if errors.Is(err, ErrInvalidReplica) {
	return fallback()
}

// Avoid
if err == ErrInvalidReplica {  // DON'T
	return fallback()
}
```

**Rationale**: `errors.Is` traverses the error wrapping chain, working correctly even when errors are wrapped.

## Type Declarations

### Struct Definition Order

1. Type declaration
2. Interface assertion checks
3. Constructor(s)
4. Helpers, ordered by least dependent first
5. Accessor methods
6. Operator methods
7. Interface implementation methods

```go
// 1. Type declaration
type DotSet struct {
	dots map[Dot]struct{}
}

// 2. Interface assertion checks
var _ DotStore = (*DotSet)(nil)

// 3. Constructor
func NewDotSet() *DotSet {
	q := &DotSet{dots: make(map[Dot]struct{})}
	return q
}

// 4. Accessors
func (p *DotSet) Has(d Dot) bool {
	_, ok := p.dots[d]
	return ok
}

// 5. Mutators
func (p *DotSet) Add(d Dot) {
	p.dots[d] = struct{}{}
}
```

### Type Aliases for Clarity

Use named types to distinguish similar primitives:

```go
type ReplicaID string
type Lattice[V any] interface {
	Join(other V) V
}
```

## Test Conventions

### Test Function Naming

Pattern: `Test{Type}_{Method}` or `Test{Type}_{Behavior}`

```go
func TestAWSet_Add(t *testing.T) { ... }
func TestAWSet_AddWinsConcurrent(t *testing.T) { ... }
func TestCausalContext_Compact(t *testing.T) { ... }
func TestJoinDotSet_Idempotent(t *testing.T) { ... }
```

### Table-Driven Tests

```go
func TestDotSet_Has(t *testing.T) {
	tcs := []struct {
		in0 Dot
		out bool
	}{
		{in0: Dot{ID: "a", Seq: 1}, out: true},
		{in0: Dot{ID: "b", Seq: 1}, out: false},
	}
	s := NewDotSet()
	s.Add(Dot{ID: "a", Seq: 1})
	for _, tc := range tcs {
		result := s.Has(tc.in0)
		if result != tc.out {
			t.Errorf("Has(%v) = %v, want %v", tc.in0, result, tc.out)
		}
	}
}
```

### Testing Framework

Use the standard `testing` package. Prefer `t.Fatalf` for early exit on unexpected state:

```go
if got := s.Len(); got != 2 {
	t.Fatalf("Len() = %d, want 2", got)
}
if !s.Has("x") {
	t.Fatal("missing x after Add")
}
```

## File Organization

### File Naming

- One primary type per file: `{type_lowercase}.go`
- Tests in parallel: `{type_lowercase}_test.go`
- Utility functions: `utils.go`

### Package Documentation

Each package contains a `CLAUDE.md` file with:
- Package overview
- File listing with descriptions
- Key types and their relationships
- Testing instructions
- Gotchas and common pitfalls

## Import Organization

Group imports: standard library first, then internal packages:

```go
import (
	"errors"
	"fmt"

	"github.com/aalpar/crdt/dotcontext"
)
```

No third-party dependencies in this project.

## Comments

Comments explain *why*, not *what*. Non-obvious logic gets context; obvious code gets none.

### Package Documentation

Use structured comments with Markdown-style headings:

```go
// Package dotcontext implements the core causal algebra for delta-state CRDTs.
//
// # Core Types
//
// Each type in this package corresponds to a mathematical structure:
//   - Dot: unique event identifier (replicaID, seq)
//   - CausalContext: compressed observed-dot set
//   - DotSet: set of dots witnessing element presence
```

### Method Documentation

- Minimal documentation for obvious methods
- Document non-obvious behavior or edge cases
- No doc comments required for standard interface implementations

### Inline Comments

- Sparse — code should be self-documenting
- Use for complex logic or non-standard patterns
- Always include space after `//`

## Constants

### Enum-like Constants

Use `iota` for sequential values:

```go
type mergeStrategy int

const (
	mergeStrategyUnknown mergeStrategy = iota
	mergeStrategyAddWins
	mergeStrategyRemoveWins
)
```

## Return Values

### No Named Returns

Functions use unnamed return values:

```go
// Correct
func (p *CausalContext) Max(id ReplicaID) uint64 {
	return p.vv[id]
}

// Avoid
func (p *CausalContext) Max(id ReplicaID) (n uint64) {
	n = p.vv[id]
	return
}
```

### Multiple Returns

Standard patterns:

```go
(V, bool)      // Value with found/ok flag
(*Type, bool)  // Concrete type with success flag
(*Type, error) // Value with possible error
```

## Miscellaneous

### Context Parameter

When needed, `ctx context.Context` is always the first parameter after receiver:

```go
func (p *Store) Load(ctx context.Context, key string) (Value, error) { ... }
```

### Builtin Shadowing

Never use Go builtin function names as local variables or parameters. Use these abbreviations:

| Builtin | Use Instead | Example |
|---------|-------------|---------|
| `len`   | `n`         | `n := len(slice)` |
| `cap`   | `c`         | `c := cap(slice)` |
| `copy`  | `cpy`       | `cpy := obj.Copy()` |

### Avoid

- Factory naming (`Create*`, `Make*` for types)
- Hungarian notation
- Excessive abbreviations beyond established patterns
- Documentation comments on trivial methods
