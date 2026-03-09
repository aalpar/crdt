# crdt

Delta-state CRDTs in Go, following [Almeida et al. 2018](https://doi.org/10.1016/j.jpdc.2017.08.003).

Zero external dependencies. All types backed by stdlib maps.

## Install

```
go get github.com/aalpar/crdt
```

## CRDTs

| Package | Type | Semantics |
|---------|------|-----------|
| `awset` | `AWSet[E]` | Add-wins observed-remove set |
| `ewflag` | `EWFlag` | Enable-wins flag |
| `lwwregister` | `LWWRegister[V]` | Last-writer-wins register |
| `pncounter` | `Counter` | Positive-negative counter |
| `ormap` | `ORMap[K, V]` | Observed-remove map with add-wins keys |

## Usage

Each CRDT follows the same pattern: mutators return a **delta** that you ship to other replicas. Replicas apply it with `Merge`.

### AWSet — add-wins set

```go
a := awset.New[string]("replica-a")
b := awset.New[string]("replica-b")

// Replica a adds "apple"; the delta goes to b.
delta := a.Add("apple")
b.Merge(delta)

// Concurrent: a removes while b adds the same element.
rmDelta  := a.Remove("apple")
addDelta := b.Add("apple")

a.Merge(addDelta)
b.Merge(rmDelta)

// Both converge to {"apple"} — add wins.
fmt.Println(a.Has("apple")) // true
fmt.Println(b.Has("apple")) // true
```

### EWFlag — enable-wins flag

```go
a := ewflag.New("replica-a")
b := ewflag.New("replica-b")

b.Merge(a.Enable()) // b learns a enabled it

// Concurrent: a disables, b enables.
a.Merge(b.Enable())
b.Merge(a.Disable())

fmt.Println(a.Value()) // true — enable wins
fmt.Println(b.Value()) // true
```

### Counter — positive-negative counter

```go
a := pncounter.New("replica-a")
b := pncounter.New("replica-b")

a.Merge(b.Increment(5))
b.Merge(a.Increment(3))
b.Merge(a.Decrement(1))

fmt.Println(a.Value()) // 7 (3 + 5 − 1)
fmt.Println(b.Value()) // 7
```

## Architecture

Two layers: a core causal algebra and higher-level CRDTs that compose it.

### Layer 1: `dotcontext/` — Core Algebra

| Type | Role |
|------|------|
| `Dot` | Unique event identifier `(replicaID, seq)` |
| `CausalContext` | Compressed observed-dot set (version vector + outliers) |
| `DotSet` | Set of dots — witnesses element presence |
| `DotFun[V]` | Dots mapped to lattice values |
| `DotMap[K, V]` | Keys mapped to nested dot stores |
| `Causal[T]` | Dot store + causal context — the unit of replication |

Join functions merge two `Causal` values. They are idempotent, commutative, and associative (semilattice properties):

| Function | Formula |
|----------|---------|
| `JoinDotSet` | `(s₁ ∩ s₂) ∪ (s₁ \ c₂) ∪ (s₂ \ c₁)` |
| `JoinDotFun` | Per-dot lattice join; unobserved dots survive |
| `JoinDotMap` | Recursive join with caller-supplied `joinV` and `emptyV` |

### Layer 2: CRDT Composition

Each CRDT wraps a `Causal[T]` and delegates conflict resolution to the join functions:

| CRDT | Internal store | Conflict rule |
|------|---------------|---------------|
| `AWSet` | `DotMap[E, *DotSet]` | Concurrent add+remove → add wins |
| `EWFlag` | `*DotSet` | Concurrent enable+disable → enable wins |
| `LWWRegister` | `DotFun[Timestamped[V]]` | Highest timestamp wins, tiebreak by replica ID |
| `Counter` | `DotFun[CounterValue]` | Sum of per-replica contributions |
| `ORMap` | `DotMap[K, V DotStore]` | Add-wins keys, recursive value merge |

### Delta Mutator Pattern

Mutators **mutate local state directly** and return a minimal delta for replication. The delta is never merged back into the source replica — doing so would misinterpret the new dot as "observed and then removed."

```
mutate local state
      │
      ▼
 build delta ──► send to remote replicas ──► remote.Merge(delta)
```

## Testing

```
go test ./...                                              # all tests
go test -race ./...                                        # race detector
go test -fuzz=FuzzJoinDotSetSemilattice ./dotcontext/      # fuzz semilattice laws
go test -bench=. -benchmem ./dotcontext/                   # benchmarks
```

## Bibliography

Papers this implementation is based on, and the work they build from.

### Primary source

**[1]** P. S. Almeida, A. Shoker, C. Baquero.
"Delta State Replicated Data Types."
*Journal of Parallel and Distributed Computing*, 111:162–173, Jan 2018.
DOI: [10.1016/j.jpdc.2017.08.003](https://doi.org/10.1016/j.jpdc.2017.08.003) · arXiv: [1603.01529](https://arxiv.org/abs/1603.01529)

### Direct precursor

**[2]** P. S. Almeida, A. Shoker, C. Baquero.
"Efficient State-Based CRDTs by Delta-Mutation."
*Networked Systems (NETYS 2015)*, LNCS 9466, pp. 62–76. Springer, 2015.
DOI: [10.1007/978-3-319-26850-7\_5](https://doi.org/10.1007/978-3-319-26850-7_5) · arXiv: [1410.2803](https://arxiv.org/abs/1410.2803)

### CRDTs coined and formalized

**[3]** M. Shapiro, N. Preguiça, C. Baquero, M. Zawirski.
"Conflict-free Replicated Data Types."
*Stabilization, Safety, and Security of Distributed Systems (SSS 2011)*, LNCS 6976, pp. 386–400. Springer, 2011.
DOI: [10.1007/978-3-642-24550-3\_29](https://doi.org/10.1007/978-3-642-24550-3_29) · [open access (HAL)](https://inria.hal.science/hal-00932836)

**[4]** M. Shapiro, N. Preguiça, C. Baquero, M. Zawirski.
"A Comprehensive Study of Convergent and Commutative Replicated Data Types."
INRIA Research Report RR-7506, 2011.
[inria.hal.science/inria-00555588](https://inria.hal.science/inria-00555588/en/)

### Dot-based causal design

**[5]** C. Baquero, P. S. Almeida, A. Shoker.
"Making Operation-Based CRDTs Operation-Based."
*Distributed Applications and Interoperable Systems (DAIS 2014)*, LNCS 8460, pp. 126–140. Springer, 2014.
DOI: [10.1007/978-3-662-43352-2\_11](https://doi.org/10.1007/978-3-662-43352-2_11)

### Observed-remove set (ancestor of AWSet)

**[6]** A. Bieniusa, M. Zawirski, N. Preguiça, M. Shapiro, C. Baquero, V. Balegas, S. Duarte.
"An Optimized Conflict-free Replicated Set."
INRIA Research Report RR-8083, 2012.
arXiv: [1210.3368](https://arxiv.org/abs/1210.3368)

### Background

**[7]** Y. Saito, M. Shapiro.
"Optimistic Replication."
*ACM Computing Surveys*, 37(1):42–81, 2005.
DOI: [10.1145/1057977.1057980](https://doi.org/10.1145/1057977.1057980)

**[8]** L. Lamport.
"Time, Clocks, and the Ordering of Events in a Distributed System."
*Communications of the ACM*, 21(7):558–565, 1978.
DOI: [10.1145/359545.359563](https://doi.org/10.1145/359545.359563)

## License

MIT
