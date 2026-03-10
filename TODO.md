# TODO

## dotcontext — core algebra

- [x] `Dot`, `CausalContext`, `DotSet`, `DotFun`, `DotMap`, `Causal` types
- [x] `CausalContext.Compact()` — fixed-point outlier promotion
- [x] `JoinDotSet` — three-term set formula
- [x] `JoinDotFun`, `JoinDotMap` — lattice join, recursive join with callback
- [x] `CausalContext.ReplicaIDs` accessor

661 tests passing across all packages (includes semilattice property checks, fuzz seed corpus).

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

## Optimization

- [x] `Compact()` was O(n²) — changed outliers from `map[Dot]struct{}` to `map[ReplicaID][]uint64` (sorted slices). Compact is now O(n), Max is O(1). (1000 outliers: 5ms→63µs, 76x speedup)
- [x] `JoinDotMap` allocation reduction — split join functions into store-only variants (`JoinDotSetStore`, `JoinDotFunStore`) that skip per-key context clone/merge/compact. (1000 keys: 16K→12K allocs, 2.0MB→1.7MB, ~20% faster)
- [x] `Merge()` O(m×n)→O(m+n) — replaced per-element BinarySearch+Insert with sorted-merge pass for outliers. (1000 interleaved outliers: 78µs→3.3µs, 23.5x)
- [x] Pre-compute single `emptyV()` in `JoinDotMap` — reuse one empty store across all key-misses instead of allocating per-key. (1000 disjoint keys: 16058→12060 allocs, −25%; 1816→1705 KB, −6%; 830→765 µs, −8%)
- [ ] Pre-sized map hints in join results — `make(map[Dot]struct{}, len(a.dots))` in `JoinDotSetStore` (and similar for `JoinDotFunStore`) when the result size is predictable from the inputs. Eliminates map growth doublings.
- [ ] In-place "merge delta into state" join path — destructive variant that mutates the large side instead of allocating a new result. The common replication case (small delta into large state) would skip the dominant allocation entirely.

## Infrastructure

- [x] CLAUDE.md
- [x] CI (GitHub Actions: build, test, vet, fmt-check, race)
- [x] Fuzz targets for join functions (found + fixed 2 bugs)
- [x] Benchmarks for CausalContext operations at scale
- [x] Makefile with build, test, bench, fuzz, lint, release targets
- [x] README.md with usage examples
- [ ] CI: run fuzz with `-fuzztime` budget on schedule (not per-push)
- [x] `go doc` comments on all exported types and functions
- [x] `crdttest/` — shared property test harness (`Harness[T]`), eliminates ~1,700 lines of duplicated test structure across 9 CRDT packages
- [x] Typed decode errors (`*DecodeLimitError`) — replaces `fmt.Errorf` in codec, enables `errors.As` for callers

## Tech debt

- [x] E2E codec wrapper boilerplate — `CausalCodec[T]` already satisfies `Codec[Causal[T]]`; removed 5 wrapper types, kept constructors returning concrete `CausalCodec` values directly.
- [x] E2E replication tests for RWSet — 3 tests: remove-wins across wire, concurrent adds across wire, re-add after remove cycle. DWFlag and GCounter still missing (lower priority — structurally identical to tested EWFlag/PNCounter).
- [x] PNCounter/GCounter near-duplication — cross-referenced both files with NOTE comments linking the shared pattern.
- [x] `DotMap.Clone()` is now deep — added `CloneStore() DotStore` to the `DotStore` interface; `DotMap.Clone()` calls `v.CloneStore().(V)` recursively.
- [x] `mvregister.Value[V]` naming — renamed to `Entry[V]` with field `Val`, consistent with `CounterValue`, `GValue`, `Presence`, `Timestamped`.
- [ ] RWSet missing per-package `TestDeltaPropagation` — every other CRDT has one; the shared harness covers it generically but the pattern break is a consistency gap.
- [ ] CRDT-level fuzz tests beyond AWSet — `awset/fuzz_test.go` found 2 bugs. RWSet (complex `Has` semantics) and ORMap (recursive join + Apply callback) are the highest-value targets.
- [ ] `DeltaStore.Fetch` scales linearly — O(|store| × |ranges|) scan. Fine while GC keeps the store small; add a per-replica secondary index if a peer-offline scenario causes buildup.
