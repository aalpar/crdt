# TODO

## dotcontext — core algebra (blocking)

- [ ] Implement `CausalContext.Compact()` — `dotcontext/context.go:73`
      Promote contiguous outliers into version vector. Loop until stable.
      Without this, `Next()` generates colliding dots.

- [ ] Implement `JoinDotSet()` — `dotcontext/join.go:12`
      Three-term set formula: `(s₁ ∩ s₂) ∪ (s₁ \ c₂) ∪ (s₂ \ c₁)`
      All other join functions depend on this.

7 tests currently failing; all are downstream of these two functions.

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
