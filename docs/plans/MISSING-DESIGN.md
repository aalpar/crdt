# CausalContext.Missing() — Anti-Entropy Primitive

## Purpose

Compute which dots a remote causal context has that the local context doesn't.
This is the fundamental anti-entropy primitive: "tell me what I'm behind on."

Pure algebra on existing types. No network, topology, or strategy dependency.

## New Type

```go
// SeqRange is an inclusive range of sequence numbers [Lo, Hi].
type SeqRange struct {
    Lo, Hi uint64
}
```

Lives in `dotcontext/` alongside `Dot`.

## Method

```go
func (p *CausalContext) Missing(remote *CausalContext) map[ReplicaID][]SeqRange
```

Returns dots that `remote` has but `p` doesn't, grouped by replica,
compressed into sorted, non-overlapping, non-adjacent ranges.

## Algorithm

Three sources of missing dots:

### 1. Version vector comparison

For each replica `id` in `remote.vv`:
- If `remote.vv[id] > p.vv[id]`, the range `{p.vv[id]+1, remote.vv[id]}` is
  missing from `p`.
- If `p` has no entry for `id`, `p.vv[id]` is 0 by Go's zero-value semantics,
  yielding `{1, remote.vv[id]}`.

This produces at most one range per replica.

### 2. Subtract p's outliers (hole-punching)

If `p` has outlier dots that fall within a VV-derived range, those dots are
already observed. Split the range around them.

Example: `p.vv["A"] = 3`, `remote.vv["A"] = 7`, `p.outliers` contains `A:5`.
Naive range: `{4, 7}`. After hole-punching: `{4, 4}, {6, 7}`.

### 3. Remote outliers

For each dot `d` in `remote.outliers`, if `!p.Has(d)`, add `d.Seq` as a
single-element range `{d.Seq, d.Seq}` for replica `d.ID`.

### 4. Merge ranges

Per replica, sort ranges by `Lo` and merge overlapping/adjacent ranges.

## Return Value Properties

- Ranges within each replica are sorted ascending by `Lo`
- Ranges are non-overlapping and non-adjacent (maximally merged)
- Empty map (or nil) means `p` is fully up-to-date with `remote`
- Neither `p` nor `remote` is mutated

## Complexity

- O(R) for version vector comparison, where R = number of replicas in remote
- O(P) for hole-punching, where P = number of outliers in p
- O(Q) for remote outlier check, where Q = number of outliers in remote
- O(K log K) for range merging per replica, where K = number of raw ranges

In the common case (well-compacted contexts, node catching up), this is
O(R) — one range per replica, no outliers to process.
