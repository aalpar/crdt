# TODO

## dotcontext — core algebra

- [x] `Dot`, `CausalContext`, `DotSet`, `DotFun`, `DotMap`, `Causal` types
- [x] `CausalContext.Compact()` — fixed-point outlier promotion
- [x] `JoinDotSet` — three-term set formula
- [x] `JoinDotFun`, `JoinDotMap` — lattice join, recursive join with callback
- [x] `CausalContext.ReplicaIDs` accessor
- [x] `CausalContext.Missing()` — anti-entropy primitive (design: `docs/plans/MISSING-DESIGN.md`)
- [x] `SeqRange` type, `MissingCodec`, fuzz round-trip
- [x] `DeltaStore.Fetch()` composable with `Missing()` return type

918 tests passing across all packages (includes semilattice property checks, fuzz seed corpus).

## Higher-level CRDTs (compose dotcontext)

- [x] `awset` — add-wins observed-remove set (`DotMap[K, *DotSet]`)
- [x] `lwwregister` — last-writer-wins register (`DotFun[TimestampedValue]`)
- [x] `pncounter` — positive-negative counter (`DotFun[CounterValue]`)
- [x] `ormap` — observed-remove map (`DotMap[K, V DotStore]`)

## New CRDT types

- [x] `mvregister` — multi-value register (concurrent writes preserved, not LWW)
- [x] `gcounter` — grow-only counter (simpler than PN, useful as building block)
- [x] `ewflag` — enable-wins flag (`DotSet` — simplest possible CRDT)
- [x] `dwflag` — disable-wins flag (complement of ewflag)
- [x] `rwset` — remove-wins observed-remove set (dual of AWSet)
- [x] `rga` — replicated growable array (`DotFun[Node[E]]`, immutable elements, tombstone ordering)
- [x] `gset` — grow-only set (no causal context; merge = set union)
- [x] `lwweset` — LWW element set (per-element timestamp; add-wins on equal timestamps)

## Optimization

- [x] `Compact()` was O(n²) — changed outliers from `map[Dot]struct{}` to `map[ReplicaID][]uint64` (sorted slices). Compact is now O(n), Max is O(1). (1000 outliers: 5ms→63µs, 76x speedup)
- [x] `JoinDotMap` allocation reduction — split join functions into store-only variants (`JoinDotSetStore`, `JoinDotFunStore`) that skip per-key context clone/merge/compact. (1000 keys: 16K→12K allocs, 2.0MB→1.7MB, ~20% faster)
- [x] `Merge()` O(m×n)→O(m+n) — replaced per-element BinarySearch+Insert with sorted-merge pass for outliers. (1000 interleaved outliers: 78µs→3.3µs, 23.5x)
- [x] Pre-compute single `emptyV()` in `JoinDotMap` — reuse one empty store across all key-misses instead of allocating per-key. (1000 disjoint keys: 16058→12060 allocs, −25%; 1816→1705 KB, −6%; 830→765 µs, −8%)
- [x] Pre-sized map hints in join results — `newDotSetSized(a.Len()+b.Len())` in `JoinDotSetStore`, `JoinDotFunStore`, and `JoinDotMap`. Eliminates map growth doublings. (1000 disjoint dots: −31% ns, −32% B, −48% allocs; DotFun 1000: −32% ns, −33% B)
- [x] In-place "merge delta into state" join path — `MergeDotSetStore`, `MergeDotFunStore`, `MergeDotMapStore` + Causal-level `MergeDotSet`, `MergeDotFun`, `MergeDotMap`. All CRDTs wired to use in-place merge. (1000 keys, 1-key delta: 353→216 µs, 921→467 KB, 7804→4776 allocs; 1.6× faster, −49% memory, −39% allocs)

## Network transport

- [x] `transport/` — `Conn` (framed `net.Conn`) + `Transport` (connection pool + `Handler` dispatch). Topology-agnostic, eager push + pull-on-connect, two message types (DeltaBatch + Ack). E2E tested with AWSet replication over localhost TCP.

## CLI

- [x] `cmd/demo` — scenario demo exercising AWSet, PNCounter, EWFlag, LWWRegister, RGA
- [x] Interactive REPL (`cmd/repl`) — all 9 CRDT types, named replicas, delta sync
  - [x] Partition simulation: `partition alice bob` / `heal alice bob`

## Infrastructure

- [x] CLAUDE.md
- [x] CI (GitHub Actions: build, test, vet, fmt-check, race)
- [x] Fuzz targets for join functions (found + fixed 2 bugs)
- [x] Benchmarks for CausalContext operations at scale
- [x] Makefile with build, test, bench, fuzz, lint, release targets
- [x] README.md with usage examples
- [x] CI: scheduled fuzz (`fuzz.yml`) — weekly, 2min/target, corpus cached across runs. Also fixed `make fuzz` to enumerate targets individually (`-fuzz` rejects multi-package and multi-match).
- [x] `go doc` comments on all exported types and functions
- [x] `crdttest/` — shared property test harness (`Harness[T]`), eliminates ~1,700 lines of duplicated test structure across 9 CRDT packages
- [x] Typed decode errors (`*DecodeLimitError`) — replaces `fmt.Errorf` in codec, enables `errors.As` for callers

## Tech debt

- [x] E2E codec wrapper boilerplate — `CausalCodec[T]` already satisfies `Codec[Causal[T]]`; removed 5 wrapper types, kept constructors returning concrete `CausalCodec` values directly.
- [x] E2E replication tests for RWSet — 3 tests: remove-wins across wire, concurrent adds across wire, re-add after remove cycle. DWFlag and GCounter still missing (lower priority — structurally identical to tested EWFlag/PNCounter).
- [x] PNCounter/GCounter near-duplication — cross-referenced both files with NOTE comments linking the shared pattern.
- [x] `DotMap.Clone()` is now deep — added `CloneStore() DotStore` to the `DotStore` interface; `DotMap.Clone()` calls `v.CloneStore().(V)` recursively.
- [x] `mvregister.Value[V]` naming — renamed to `Entry[V]` with field `Val`, consistent with `CounterValue`, `GValue`, `Presence`, `Timestamped`.
- [x] RWSet missing per-package `TestDeltaPropagation` — every other CRDT has one; the shared harness covers it generically but the pattern break is a consistency gap.
- [x] CRDT-level fuzz tests beyond AWSet — `awset/fuzz_test.go` found 2 bugs. Added `rwset/fuzz_test.go` (convergence) and `ormap/fuzz_test.go` (convergence + nested 3-level recursive merge).
- [x] `DeltaStore.Fetch` scales linearly — O(|store| × |ranges|) scan. Added per-replica sorted secondary index (`byReplica map[ReplicaID][]uint64`); Fetch is now O(Σ_r |ranges_r|×log(|dots_per_r|) + |hits|). Tail fetch of 100/1000 dots: ~4.7µs vs ~59µs for full scan (12.5× on the offline-peer scenario).
- [x] RGA tombstone GC — `PurgeTombstones(canGC func(Dot) bool) int` removes tombstoned entries, retaining phantom After pointers (`gcAfter map[Dot]Dot`) to preserve linearization order. Naive re-parenting breaks sibling sort; phantoms keep the tree identical. 9 tests including concurrent-insert order preservation. Known limitation: `gcAfter` not included in serialized `State()` — full state transfer needs re-parenting or gcAfter serialization.
- [x] `gcAfter` phantom compaction — `CompactPhantoms()` removes unreferenced phantoms via refcount + work queue (O(N), cascading). Caller ensures all peers are caught up before compacting (operator decision, not automatic). 6 tests including cascade chain removal.
- [x] RGA `State()` serialization with phantoms — `Snapshot()` returns `Snapshot[E]` (Causal state + phantom map). `SnapshotCodec` encodes phantoms as `[uint64 count] [Dot Dot]...` pairs. `FromSnapshot()` restores full state. 4 round-trip tests including phantom chain survival.
