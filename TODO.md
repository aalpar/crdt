# TODO

## dotcontext — core algebra

- [x] Implement `CausalContext.Compact()` — fixed-point outlier promotion
- [x] Implement `JoinDotSet()` — three-term set formula
- [x] `JoinDotFun`, `JoinDotMap` — lattice join, recursive join with callback

31 tests passing (includes semilattice property checks).

## Higher-level CRDTs (composes dotcontext)

- [ ] `awset` — add-wins observed-remove set (`DotMap[K, *DotSet]`)
- [ ] `lwwregister` — last-writer-wins register (`DotFun[TimestampedValue]`)
- [ ] `pncounter` — positive-negative counter (`DotFun[CounterValue]`)
- [ ] `ormap` — observed-remove map (`DotMap[K, V DotStore]`)

## Infrastructure

- [ ] CLAUDE.md for the crdt project
- [ ] CI (GitHub Actions: build, test, vet)
- [ ] Fuzz targets for join functions
- [ ] Benchmarks for CausalContext operations at scale
