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

- [ ] CLAUDE.md for the crdt project
- [x] CI (GitHub Actions: build, test, vet, fmt-check, race)
- [x] Fuzz targets for join functions (found + fixed 2 bugs)
- [ ] Benchmarks for CausalContext operations at scale
