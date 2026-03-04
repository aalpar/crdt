# TODO

## dotcontext — core algebra

- [x] Implement `CausalContext.Compact()` — fixed-point outlier promotion
- [x] Implement `JoinDotSet()` — three-term set formula
- [x] `JoinDotFun`, `JoinDotMap` — lattice join, recursive join with callback

31 tests passing (includes semilattice property checks).

## Higher-level CRDTs (composes dotcontext)

- [x] `awset` — add-wins observed-remove set (`DotMap[K, *DotSet]`)
- [x] `lwwregister` — last-writer-wins register (`DotFun[TimestampedValue]`)
- [x] `pncounter` — positive-negative counter (`DotFun[CounterValue]`)
- [x] `ormap` — observed-remove map (`DotMap[K, V DotStore]`)

## Infrastructure

- [x] CLAUDE.md for the crdt project
- [x] CI (GitHub Actions: build, test, vet, fmt-check, race)
- [x] Fuzz targets for join functions (found + fixed 2 bugs)
- [x] Benchmarks for CausalContext operations at scale

## Optimization

- [ ] `Compact()` is O(n²) — fixed-point loop iterates all outliers per pass, needs up to n passes. Sorted-insert approach could bring to O(n log n). (10→1.3µs, 100→57µs, 1000→5ms)
- [ ] `JoinDotMap` at 1000 keys allocates 16K objects — each key clones DotSets for the join formula

## New CRDT types

- [ ] `mvregister` — multi-value register (concurrent writes preserved, not LWW)
- [ ] `gcounter` — grow-only counter (simpler than PN, useful as building block)
