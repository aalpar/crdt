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
