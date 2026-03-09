# crdt

Delta-state CRDTs in Go, following Almeida et al. 2018.

## Versioning

- v0.x with zero consumers. Break freely — no stability guarantees.

## Architecture

Two layers: a zero-dependency core algebra and higher-level CRDTs that compose it.

### Layer 1: `dotcontext/` — Core Algebra

Zero external dependencies. All types backed by stdlib maps.

| Type | Purpose |
|------|---------|
| `Dot` | Unique event identifier (replica, seq) |
| `CausalContext` | Compressed observed-dot set (version vector + outliers) |
| `DotSet` | Set of dots — `P(I × N)` |
| `DotFun[V Lattice[V]]` | Dots mapped to lattice values |
| `DotMap[K, V DotStore]` | Keys mapped to nested dot stores |
| `Causal[T DotStore]` | Dot store + causal context (unit of replication) |

Join functions merge two `Causal` values (idempotent, commutative, associative):

| Function | Formula |
|----------|---------|
| `JoinDotSet` | `(s₁ ∩ s₂) ∪ (s₁ \ c₂) ∪ (s₂ \ c₁)` |
| `JoinDotFun` | Per-dot lattice join, unobserved dots survive |
| `JoinDotMap` | Recursive join with caller-supplied `joinV` and `emptyV` |

### Layer 2: Higher-Level CRDTs

Each composes dotcontext types. Mutators return deltas for replication.

| Package | Type | Composition | Conflict Resolution |
|---------|------|-------------|-------------------|
| `awset/` | `AWSet[E]` | `DotMap[E, *DotSet]` | Concurrent add+remove → add wins |
| `rwset/` | `RWSet[E]` | `DotMap[E, *DotFun[Presence]]` | Concurrent add+remove → remove wins |
| `lwwregister/` | `LWWRegister[V]` | `DotFun[Timestamped[V]]` | Highest timestamp wins, tiebreak by replica ID |
| `pncounter/` | `Counter` | `DotFun[CounterValue]` | Sum of per-replica contributions |
| `gcounter/` | `Counter` | `DotFun[GValue]` | Sum of per-replica contributions (grow-only) |
| `ormap/` | `ORMap[K, V]` | `DotMap[K, V DotStore]` | Add-wins keys, recursive value merge |
| `ewflag/` | `EWFlag` | `Causal[*DotSet]` | Concurrent enable+disable → enable wins |
| `dwflag/` | `DWFlag` | `Causal[*DotSet]` | Concurrent enable+disable → disable wins |

### Key Design Decisions

- Mutators generate dots from the main `CausalContext` and mutate local state directly — no self-merge (avoids false "observed and removed" interpretation)
- `JoinDotMap` takes `emptyV func() V` parameter (not a type switch) so it works with arbitrary `DotStore` types
- `math/big` is not used here (that's the shamir project); this project uses stdlib maps throughout
- `Compact()` removes outliers at or below the version vector frontier, not just contiguous ones

## Package Map

| Package | Key Types | Files |
|---------|-----------|-------|
| `dotcontext/` | Dot, CausalContext, DotSet, DotFun, DotMap, Causal | 11 source + 10 test |
| `awset/` | AWSet | 2 source + 2 test |
| `rwset/` | RWSet, Presence | 2 source + 1 test |
| `lwwregister/` | LWWRegister | 2 source + 1 test |
| `pncounter/` | Counter | 2 source + 1 test |
| `gcounter/` | Counter, GValue | 2 source + 1 test |
| `ormap/` | ORMap | 2 source + 1 test |
| `ewflag/` | EWFlag | 2 source + 1 test |
| `dwflag/` | DWFlag | 2 source + 1 test |
| `replication/` | PeerTracker, GC, WriteDeltaBatch | 4 source + 4 test |

## Testing

- `go test ./...` — 644 tests across all packages
- `go test -race ./...` — race detector
- `go test -fuzz=FuzzJoinDotSetSemilattice ./dotcontext/` — fuzz semilattice properties
- `make benchmark` — benchmarks across all packages
- `make profile` — CPU + memory profiles to `profiles/` (default: `dotcontext/`, override: `PROF_PKG=./awset/ make profile`)
- **After changes**: `make lint && make && make test`

## Document Naming

Files in `docs/plans/`:

- **Permanent** — `ALL-CAPS-AND-DASHES.md`. Long-lived design docs that stay current.
- **Ephemeral** — `YYYY-MM-DD-lowercase-name.md`. Committed at least once (recoverable via git history), may not survive beyond one commit. Implementation plans, scratch designs, session notes.

## Commits

- Direct push to master is fine at this stage
- No "Co-Authored-By" lines
