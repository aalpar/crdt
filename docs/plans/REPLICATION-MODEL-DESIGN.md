# Replication Model Design

## Problem Statement

The CRDT algebra (`dotcontext/`) guarantees convergence: any delivery order
produces the same result. But the algebra says nothing about *how* deltas move
between replicas. This document defines the replication model — the strategy
layer that sits between the CRDT algebra and the network.

## The Ideal

A perfect system would make all information visible to all participants
instantly, regardless of network topology or condition. Every design decision
in this document is a measurable departure from that ideal, forced by physical
constraints:

| Constraint         | What it prevents                                    |
|--------------------|-----------------------------------------------------|
| Finite bandwidth   | Can't send everything to everyone simultaneously    |
| Finite storage     | Not every node can hold all data                    |
| Finite availability| Nodes go offline; some paths are temporarily gone   |
| Speed of light     | Latency floor between distant nodes                 |

The system optimizes along four axes:

- **Broadly** — data available to anyone who wants it
- **Efficiently** — don't waste resources replicating data nobody has requested
- **Consistently** — any piece of data survives node failures (durability)
- **Quickly** — minimize propagation delay from writer to interested readers

These are in tension. Broadly vs. efficiently: replicating everywhere maximizes
availability but wastes resources. Consistently vs. efficiently: more copies
means more durable but more costly. The strategy layer navigates these tensions
given the physical constraints of the actual network.

## The Cost of State

Every byte of state a node carries has measurable costs:

| Cost               | Measure                          | Scales with                   |
|--------------------|----------------------------------|-------------------------------|
| Memory             | Bytes stored                     | State size                    |
| Bandwidth          | Bytes transmitted per sync       | Delta size × peer count       |
| Merge time         | Operations per join              | State elements examined       |
| Failure exposure   | Data at risk on node crash       | State size on failed node     |

In a replicated system, these costs multiply. One redundant byte replicated to
k peers costs k bytes of storage, k bytes of bandwidth, and k merge-time
contributions. The replication factor is a cost multiplier on every piece of
state. Minimizing state is not an optimization — it is a constraint with the
same standing as bandwidth or availability.

### Irreducible Minimums

**Causal metadata.** Tracking causality among n replicas requires at least O(n)
metadata — a proven lower bound (Charron-Bost, 1991). The `CausalContext`
version vector meets this bound. The outlier set is variable overhead above the
minimum; `Compact()` reduces it toward the floor.

**Replication payload.** Delta-state CRDTs (Almeida et al., 2018) transmit only
what changed since the last sync, not the full state. The anti-entropy
mechanism (Demers et al., 1987) computes the minimal diff between two peers.
Together, these ensure the bytes shipped approach the information-theoretic
minimum: only novel information crosses the wire.

### Design Implications

Every design decision in this document is evaluated against state cost:

- **Partial replication** — no node carries content it doesn't need. Full
  replication is reserved for the n-k subsystem where security demands it.
- **Metadata vs. content split** — metadata (small, O(n)) is fully replicated;
  content (large) is partially replicated. The split exists because the costs
  differ by orders of magnitude.
- **Shared infrastructure** — security, durability, and replication use the
  same k-of-n nodes and the same replication traffic. Three functions, one
  data structure. Separate structures for each would be redundant weight.
- **Eager push, not full-state sync** — a delta is smaller than a snapshot.
  Eager delta push pays less bandwidth tax than periodic full-state exchange.
- **No coordination state** — CRDTs converge without consensus. The system
  carries no leader election state, no Paxos ballots, no Raft logs. That
  entire category of state — and its replication cost — is absent by
  construction.

A redundant data structure is not merely inelegant — it is a measurable cost
at every replica, on every sync, for the lifetime of the system.

## Architecture

```
┌─────────────────────────────────────────────┐
│              Control Plane                  │
│  Distributed trust authority (k-of-n)       │
│  Issues credentials, manages membership     │
│  Threshold: requires k servers              │
│  Frequency: rare (join, revoke, renew)      │
├─────────────────────────────────────────────┤
│               Data Plane                    │
│  CRDT deltas between authenticated peers    │
│  No quorum, no coordination                 │
│  Eager push from fragile nodes              │
│  Durability from local peer count           │
│  Frequency: every write                     │
├─────────────────────────────────────────────┤
│             CRDT Algebra                    │
│  dotcontext, awset, ormap, ...              │
│  Convergence guaranteed regardless of       │
│  delivery order or topology                 │
└─────────────────────────────────────────────┘
```

The algebra is the bottom layer — already built. The data plane is the strategy
layer. The control plane secures it. The two planes do not block each other.

## Partial Replication

Full replication is impossible. The total data in the system exceeds the
capacity of any single node. No node holds everything. This is not a design
choice — it is a hard constraint of the problem space.

Every node holds a subset of the total data. But the entire system is viewable:
any piece of data is accessible, though not all data is local. This creates
two kinds of reads:

| Read type | Data location | Requires network? | Availability     |
|-----------|---------------|-------------------|------------------|
| Local     | On this node  | No                | Always available |
| Remote    | On a peer     | Yes               | Requires connectivity |

### Metadata vs. Content

This constraint motivates a separation between metadata and content:

- **Metadata** (causal contexts, version vectors) — small. Tracks *which events
  exist* and their causal relationships. Can be fully replicated across all
  nodes.

- **Content** (dot store entries, actual values) — large. The data itself.
  Partially replicated based on node capacity and interest.

Every node knows what data exists in the system (metadata is fully replicated).
Not every node has the data itself (content is partially replicated). A node
can answer "do I have this?" instantly, and "where can I get it?" using peer
knowledge.

This separation means the CRDT algebra operates on two levels:

- **Metadata merges** happen eagerly between all peers — keeping every node's
  causal context up to date with the full system state.
- **Content transfers** happen on demand or by interest — a node pulls content
  it needs from peers that have it.

The eager-push-upward rule applies to both: a client pushes its content to a
server (durability), and metadata propagates everywhere (awareness). The
lazy-pull-downward rule applies to content: a client pulls content it needs
when it needs it.

## Nodes

A node is any device that participates in the system. Nodes are not assigned
roles — their behavior emerges from measurable properties:

| Property      | Meaning                                | Example range          |
|---------------|----------------------------------------|------------------------|
| Bandwidth     | Sustained throughput to peers           | 1 Mbps – 10 Gbps      |
| Availability  | Fraction of time reachable             | 0.3 (phone) – 0.999   |
| Storage       | Capacity for replica state             | 1 GB – 10 TB          |
| Participants  | Number of users this node serves       | 1 – 10,000+           |

These properties are correlated: a node serving many participants must invest
in bandwidth, availability, and storage to sustain them. A node serving one
person may have minimal investment in all dimensions.

## Capabilities

Nodes compose three orthogonal capabilities:

### Client

A node with a user. Generates writes. Typically low availability — comes and
goes with the user.

### Server

A node with high availability, high bandwidth, and a stable address. Receives
eager pushes from clients. Natural trust anchor (see Control Plane below).

### Peer

A node with symmetric replication relationships to neighbors. Peer
relationships are always bidirectional: if A peers with B, then B peers with A.

These compose freely:

| Combination     | Description                                           |
|-----------------|-------------------------------------------------------|
| Client only     | Writes locally, pushes to server, no lateral exchange |
| Client + Peer   | Writes locally, pushes to server, replicates with neighbors |
| Server + Peer   | Receives client pushes, replicates with other servers |
| Peer only       | No user, no client pushes — just replicates with neighbors |

The model does not fix a specific number of tiers. Property thresholds may
demand tier-specific software (e.g., a "metaserver" or "subclient"), but the
model accommodates any number of tiers without modification.

## Data Plane

### Push Policy: Data as Gravity

Data flows toward higher availability. Each node pushes deltas toward the most
available peers it knows about. This is push-based epidemic replication (Demers
et al., 1987) directed by the availability gradient:

- **Eager push from fragile nodes is mandatory.** A low-availability node (a
  client) is a sinking ship. Its data must reach a high-availability node
  before it goes offline. The window is unknown, so push immediately.

- **Eagerness is proportional to fragility.** The less available a node, the
  more urgently it must push. High-availability nodes can afford to let peers
  pull, but eager push is the default for all peers.

- **A node that cannot reliably relay is not used as a relay.** A client cannot
  be a reliable intermediary — it may go offline before forwarding. The
  strategy routes around low-availability nodes.

### The "Ready" Gate

Not all local data is immediately available for replication. A user may have
uncommitted work — half-finished edits that should not propagate. The
application controls when data enters the replication layer:

- **Local-only** — exists on the node, not offered for replication.
- **Ready** — offered for replication. Eager push begins.

This is a local decision by the user or application. The strategy layer never
reaches into the CRDT — it only handles deltas it has been given. The interface
between the CRDT layer and the strategy layer is a queue of ready deltas, each
tagged with their causal context.

### Durability

A node does not need to know the total membership of the system to reason about
durability. It only needs to know:

- How many peers have accepted this delta (local count)
- How available those peers are (local knowledge)

This is sufficient to compute a durability estimate:

```
P(write survives) = 1 - P(all copies lost)
                  = 1 - ∏(1 - availability_i) for i in {originator, peers...}
```

Example: a phone (availability 0.3) pushes to one server (availability 0.999):

```
P(survive) = 1 - (0.7)(0.001) = 0.9993
```

No global membership knowledge. No voting. The system provides a per-node,
per-write durability estimate based on local information.

#### Quorum-Confirmed Durability

The control plane already requires k-of-n server coordination for trust
operations (see Control Plane below). Since that infrastructure exists, the
data plane can leverage it to upgrade durability from a probabilistic estimate
to a binary confirmation:

```
Write locally           → always succeeds (CRDT, no coordination)
Push to peers           → eager, best-effort
k peers acknowledge     → write is confirmed durable
fewer than k available  → write exists but durability unconfirmed
```

The critical property: **quorum for confirmation, not for permission.** A node
always writes locally — that never blocks. The quorum determines when a write
is *confirmed* durable, not whether the write is *allowed*. Below-threshold
means unconfirmed, not failed. The system never deadlocks.

This is the same degradation model as the control plane: below-threshold means
reduced guarantees, not system failure. A write in "unconfirmed" state is still
local, still replicating via eager push, still subject to the probabilistic
durability estimate above. Confirmation is a stronger signal layered on top,
not a replacement.

The argument for using quorum on data: since security already demands k-of-n
server coordination, the marginal cost of data durability confirmation is low.
The infrastructure is shared. If the system must tolerate quorum unavailability
for security (no new credentials when fewer than k servers are reachable), it
can tolerate the same for data confirmation (no durability confirmation when
fewer than k peers have acknowledged).

### No Quorum for Writes

The system never requires N nodes to agree before making progress. A single
node can always read and write locally. Durability improves with more peers
and can be confirmed via quorum acknowledgment, but the system never deadlocks
on membership. Writes are always local. Quorum is an optional confirmation
layer, not a gate.

This is fundamentally different from consensus-based systems — Paxos (Lamport,
1998) and Raft (Ongaro & Ousterhout, 2014) — where below-quorum means hard
stop. Here, fewer peers means lower (or
unconfirmed) durability, not system failure. Degradation is graceful.

This property is essential for networks of unreliable peers. In such a network,
total membership is unknowable at any moment. No census is possible. A system
that required quorum for writes would deadlock because the threshold cannot be
computed.

### Topology Is Emergent

No topology is chosen a priori. The set of all peer relationships defines the
replication graph. The shape of that graph — ring, tree, mesh, or hybrid —
emerges from:

- The properties of the nodes (bandwidth, availability, storage)
- The strategy's decision function (push toward higher availability)
- The peer discovery mechanism (who knows about whom)

Different distributions of node properties produce different topologies:

- When one node has vastly higher availability than all others: star
- When availability is spread across orders of magnitude: tree
- When all nodes have similar properties: mesh or ring
- In practice: a hybrid driven by the actual network

The CRDT algebra guarantees correctness regardless of which topology emerges.
The strategy layer is free to optimize for speed and efficiency without
compromising convergence.

## Peerage

Peerage is the primitive that provides durability through redundancy. Two nodes
that replicate to each other are peers.

### Properties

- **Symmetric.** If A peers with B, then B peers with A. The relationship is
  always bidirectional.

- **Authenticated.** Peers must verify each other's identity before exchanging
  deltas. Trust is established via credentials issued by the control plane.

- **Peer count determines durability.** More peers with copies of a delta means
  more nodes must fail before the data is lost. One high-availability peer is
  worth more than many low-availability peers.

- **Peer-to-peer push is eager by default.** Peerage exists for durability.
  Lazy pull between peers defeats the purpose — if peer B waits to request
  from peer A, and A dies first, the data is gone.

### Anti-Entropy

Two peers synchronize using causal context comparison. The `CausalContext`
already supports this: compare version vectors, compute the diff, send only
the missing deltas. This is anti-entropy synchronization (Demers et al., 1987)
applied to delta-state CRDTs (Almeida et al., 2018), and the machinery exists
in `dotcontext/`.

Anti-entropy between known peers is **direct**, not gossip-based. In the n-k
subsystem, servers know each other — it is a configured membership group.
Randomized gossip solves a discovery/scalability problem that does not exist
here. Direct anti-entropy via `Missing()` + `Fetch()` between known peers is
simpler, deterministic, and complete.

### Delta Delivery

Deltas move between peers through two complementary mechanisms:

**Eager push buffer.** When a mutator fires, the delta is stored in an
in-memory `DeltaStore` indexed by the dot that created it. The delta is then
pushed to known peers. The buffer is ephemeral — lost on crash.

**Anti-entropy fallback.** When a peer reconnects after being offline or after
a crash that lost the buffer, it exchanges `CausalContext` summaries with its
peers. `Missing()` computes what dots the peer needs; the current CRDT state
(not the buffer) provides the data. Anti-entropy is the primary recovery
mechanism and the source of correctness.

The buffer is an optimization for fast propagation. Anti-entropy is the
guarantee of eventual convergence. The buffer can always be lost without
affecting correctness.

One delta per dot. Remove operations (AWSet.Remove, EWFlag.Disable) produce
deltas with no new dots and are not indexed in the delta store. Their delivery
is handled by the anti-entropy mechanism, which propagates causal context
advancement.

### GC Policy

A delta in the eager push buffer is safe to discard when it is no longer
needed for delivery. The GC threshold is **n** — all servers in the n-k
subsystem must have acknowledged the delta before it is removed from the
buffer.

Why n, not k:

- **The buffer is cheap.** In-memory, small deltas, short-lived. Holding
  deltas slightly longer has negligible cost.
- **k < n leaves a gap.** If the buffer is GC'd at k and then the node
  crashes, the remaining k-1 servers have the data but may not have propagated
  it to the other n-k servers yet. Anti-entropy would eventually fill the gap,
  but why introduce a window of reduced durability when the buffer is cheap?
- **n is simple.** Remove from buffer when all peers have acknowledged. No
  probabilistic reasoning about "enough copies."

An "already have it" acknowledgment counts the same as a fresh delivery
acknowledgment. Both prove the peer has the data, which is what matters for
durability.

Client nodes that are not part of the n-k subsystem follow the same principle:
GC when all known peers have acknowledged. For a client with 2 server peers,
that means both servers must ACK.

## Discovery

A node must find at least one peer before peerage is useful. Three mechanisms,
which coexist:

| Mechanism           | Range               | Configuration              |
|---------------------|---------------------|----------------------------|
| Broadcast           | Local network (LAN)  | None                       |
| Configured address  | Global (internet)    | One address required       |
| Introduction        | Grows from above     | None beyond initial peer   |

**Broadcast** — zero-configuration discovery on the local network via UDP
broadcast. A phone discovers a laptop on the same wifi. Bounded by network
segment; does not cross the internet. Storm mitigation via jittered beacon
intervals (see Transport section).

**Configured address** — a node is told the address of a peer (typically a
server). Works globally. Requires out-of-band knowledge — someone must provide
the address. The n-k servers use configured addresses to find each other.

**Introduction** — if A knows B, and B knows C, B can introduce A to C.
Requires one bootstrap connection; from there, the peer set grows
organically. Introductions are authenticated: the introducing peer vouches
for the new peer using credentials from the control plane.

A server is a natural bootstrap point: always available, stable address,
discoverable. A client configures one server address, connects, and learns
about other peers through introductions.

## Transport

Two protocols, split by purpose:

### TCP — Bulk Data

Delta payloads, full state transfer (anti-entropy catch-up for big gaps), and
anything that might exceed the MTU. TCP provides reliability, ordering, and
flow control for data that must arrive intact. Used for WAN and LAN.

### UDP — Signals

Small, ACK-like messages and state transitions. Broadcast-compatible.
Reliable in the narrow sense: the sender knows whether the message was
received. All payloads designed to fit in a single UDP datagram (well under
MTU).

Messages in this category:

| Message                     | Broadcast? | ACK needed?                            |
|-----------------------------|------------|----------------------------------------|
| Discovery beacon            | Yes        | No — periodic, next one comes soon     |
| CausalContext summary       | No         | No — response IS the ACK               |
| "I have new data" notify    | No         | No — next anti-entropy round catches it|
| GC state ("I have your dots")| No        | No — if lost, sender holds buffer longer|

#### Design Principles

**Zero-ACK by default.** Every message is designed so that silence is safe. No
response means "try again later." The system converges regardless because
CRDTs make all timeouts forgiving.

**Idempotent messages.** Duplicate delivery is harmless. Sequence numbers
detect duplicates but processing a duplicate has no side effects.

**Long backoffs.** Anything that might need retransmit must wait. Exponential
backoff with long maximum ceiling. The UDP protocol is for signals, not urgent
data — patience is built in.

**No broadcast for notifications.** Discovery beacons are broadcast.
Everything else is unicast to known peers. This prevents ACK implosion (a
broadcast triggering N simultaneous responses).

#### Anti-Entropy Initiation

Anti-entropy starts over UDP: a node sends its CausalContext summary (version
vector + outliers) to a peer. The summary is O(n) where n is the replica
count — well under MTU for any reasonable n.

The peer runs `Missing()` against the received context. If the result is
empty, no further action — the peers are synced. If non-empty, the peer
opens a TCP connection to fetch the missing deltas. This avoids TCP connection
overhead in the common case (peers already synced).

#### Storm Mitigation

- **Discovery storms on join:** jittered beacon intervals — each node waits a
  random delay before its first beacon.
- **ACK implosion:** prevented by design — notifications are unicast, not
  broadcast. The peer list is bounded and known.
- **Retransmission pileup:** idempotent messages + sequence numbers make
  duplicates harmless. Exponential backoff prevents retransmission floods.

## Control Plane

### Distributed Trust Authority

The most reliable trust systems are asymmetric: one party has authority the
other doesn't (CA signs certificates, clients present them). This asymmetry
maps directly to the capacity gradient:

- A trust anchor must be always available (can't verify credentials against an
  offline node), discoverable (stable address), and durable (loss of the anchor
  breaks the trust chain).
- These are exactly the properties of a server.

The server role therefore has three correlated, mutually reinforcing properties:

1. **High availability** — always on
2. **Discoverable** — stable address
3. **Trust anchor** — issues and verifies credentials

These are not independent design choices. High availability enables
discoverability, which enables trust anchoring. The role is not assigned — it
is forced by the physics of trust.

### Threshold Trust

A single trust anchor is a single point of failure for the control plane (even
if the data plane is fully decentralized). The trust authority is therefore
distributed across multiple servers using threshold secret sharing (Shamir,
1979):

- k-of-n servers must participate to issue credentials
- No single server's failure breaks the trust chain

### The n-k Subsystem

Distributed security requires a designated subset of nodes to participate in
threshold operations. This subset — the n-k subsystem — is a tightly-coupled
replication group with specific properties:

**Full replication within the group.** Unlike the broader system where partial
replication is the norm, n-k nodes fully replicate each other. Every node in
the subsystem holds all data that the subsystem is responsible for. This serves
both durability (k copies exist) and security (k nodes can participate in
threshold operations over the data).

**Structured replication traffic.** Between n-k peers, replication is
predictable and optimizable:

| Property           | n-k peer replication    | User-facing traffic      |
|--------------------|-------------------------|--------------------------|
| Peers              | Known, fixed set        | Unknown, any client      |
| Operations         | Context diff → deltas   | Arbitrary reads/writes   |
| Predictability     | O(1) per operation      | O(?)                     |
| Scheduling         | Continuous, structured  | Bursty, unstructured     |

n-k systems spend most of their bandwidth on peer replication. This is not a
flaw — it is the primary function. Replication between known peers with known
state is cheap and predictable. User requests are not.

**Shield the subsystem from unstructured traffic.** Mixing structured
replication with unstructured user requests degrades both. The n-k subsystem
should be optimized for its primary function (replication and threshold
operations). User-facing traffic should be served by nodes that sit in front
of the n-k core and proxy requests as needed.

This creates a concrete tier boundary forced by operational reality:

- **n-k core**: fully replicated, optimized for peer replication and threshold
  operations, limited user-facing load
- **Edge nodes**: serve users, push writes to the core eagerly, pull content
  from the core on demand

This is not a client-server split imposed by design — it is forced by the
requirement that security demands threshold coordination, and threshold
coordination demands dedicated replication bandwidth.

### Shared Infrastructure

The k-of-n coordination required for trust operations, the full replication
required for the n-k subsystem, and the quorum-confirmed durability on the
data plane are all the same infrastructure. The n-k subsystem serves all three
purposes simultaneously:

- **Security**: threshold trust operations (credential issuance, revocation)
- **Durability**: k acknowledged copies = confirmed durable
- **Replication**: full internal replication keeps all n-k nodes in sync

Since security demands this infrastructure regardless, the incremental cost
of data durability confirmation is zero — it falls out of the replication
that the n-k subsystem already performs. The system pays the threshold tax
once and uses it for all three purposes.

### Constraints the n-k Subsystem Imposes

Security paints the system into supporting an n-k subsystem. This is not
optional — distributed trust requires it. Once that requirement exists, the
n-k subsystem imposes constraints on the larger system:

- **The n-k nodes must be provisioned.** They need high availability, high
  bandwidth, and sufficient storage for full replication. The broader system
  depends on their existence.
- **The threshold k must be chosen.** Too low and security is weak (few nodes
  needed to compromise trust). Too high and the subsystem is fragile (many
  nodes must be available for threshold operations).
- **Edge nodes depend on the n-k core.** Credential issuance, durability
  confirmation, and content availability all flow through the core. The edge
  is autonomous for local operations but dependent on the core for guarantees.

Since the n-k subsystem exists regardless, the system should explore what else
it enables. Quorum-confirmed durability (above) is one example. Other quorum
models may offer additional properties — alternative quorum structures could
provide different trade-offs between availability, load distribution, and
fault tolerance.

### Two-Plane Independence

The control plane and data plane do not block each other. Both use the shared
k-of-n infrastructure, but neither requires it for basic operation:

| Operation              | Plane    | Requires k-of-n? | Degrades to          |
|------------------------|----------|-------------------|----------------------|
| Local read/write       | Data     | No                | Always available     |
| Delta exchange         | Data     | No (peers only)   | Always available     |
| Durability confirmation| Data     | Yes (k-of-n)      | Unconfirmed writes   |
| New node onboarding    | Control  | Yes (k-of-n)      | Deferred until quorum|
| Credential revocation  | Control  | Yes (k-of-n)      | Deferred until quorum|
| Credential renewal     | Control  | Yes (k-of-n)      | Deferred until quorum|

A node with valid credentials can read, write, and replicate even if every
server is unreachable. Writes continue; durability confirmation and membership
changes wait for quorum to become available. The system never stops — it
offers reduced guarantees until the infrastructure recovers.

## Failure Detection and Timeouts

Timeouts are central to the design. Every non-Byzantine failure — crashed node,
network partition, full disk — manifests identically from a peer's perspective:
the node stopped responding. The only detection mechanism is to wait some
duration and declare failure.

### Why Logical Time Cannot Replace Wall Clocks

Logical clocks (Lamport, 1978) and version vectors track causality — event A
happened before event B. They say nothing about duration. A CausalContext at
`(alice:5, bob:3)` records which events have been observed, not when they
happened or how long ago. You cannot express "wait 10 Lamport ticks" because
Lamport ticks advance only when events occur. If no events occur, they don't
advance.

This is a consequence of the FLP impossibility result (Fischer, Lynch &
Paterson, 1985): in an asynchronous system, it is impossible to distinguish a
slow node from a dead one. No amount of logical-time machinery changes this.
Wall-clock measurement is irreducible.

### Why It Matters Less in CRDTs

In consensus systems (Paxos, Raft), a timeout triggers leader election — a
correctness-critical operation. A wrong timeout causes split brain or liveness
failure. The timeout must be right.

In a CRDT system, a timeout triggers nothing correctness-critical. Convergence
is guaranteed regardless of delivery order, timing, or failures. A timeout
means "stop waiting for this peer, try another" or "update this peer's
availability estimate." Getting it wrong wastes time or bandwidth but never
breaks correctness.

**Timeouts in this system are optimization hints, not correctness requirements.**

This does not mean timeouts can be hand-waved. The durability estimate, peer
selection, and push scheduling all depend on accurate failure detection. A
too-long timeout means continuing to push to a dead peer. A too-short timeout
means prematurely abandoning a slow but healthy peer. Both degrade performance.

### The Phi Accrual Failure Detector

The most principled approach to adaptive failure detection is the phi (φ)
accrual failure detector (Hayashibara et al., 2004). Instead of binary
alive/dead with a fixed threshold:

1. Maintain a sliding window of inter-arrival times from each peer.
2. Compute a suspicion level φ based on how long since the last message,
   relative to the observed distribution of inter-arrival times.
3. Higher φ = more suspicious. The application chooses its own threshold.

```
φ = -log₁₀(1 - F(t_now - t_last))

where F is the CDF of the observed inter-arrival distribution
and t_last is the timestamp of the last received message
```

Key properties:

- **Adaptive.** Automatically adjusts to each peer's actual behavior. A phone
  that responds every 30 seconds gets a different distribution than a server
  that responds every 100ms.
- **Continuous.** Suspicion is a scalar, not binary. Different operations can
  use different thresholds: a durability estimate might tolerate φ = 3 (low
  confidence of failure), while peer eviction might require φ = 8 (high
  confidence).
- **Per-peer.** Each peer relationship maintains its own arrival window and
  distribution. No global timeout value.
- **Production-proven.** Used in Cassandra and Akka.

### CRDT-Specific Optimizations

The causal context carries implicit liveness information:

- **Deltas are heartbeats.** During active replication, the arrival of new
  deltas from a peer is direct evidence of liveness. No dedicated heartbeat
  protocol is needed while data is flowing.
- **Heartbeats are only needed during quiet periods.** When a peer has no new
  deltas, a lightweight keepalive confirms liveness without data overhead.
- **Node properties inform initial estimates.** A peer with declared
  availability 0.3 should start with a much wider inter-arrival distribution
  than one with availability 0.999. The phi detector adapts regardless, but
  good initial estimates reduce the warm-up period.

### Timeout Policy

The failure detector informs but does not dictate policy. Different operations
use the suspicion level differently:

| Operation              | Suspicion threshold | Consequence of wrong call       |
|------------------------|--------------------|---------------------------------|
| Push peer selection    | Low (φ ~ 1-2)      | Waste bandwidth on likely-dead peer |
| Durability estimation  | Medium (φ ~ 3-5)   | Over/underestimate write safety |
| Peer eviction          | High (φ ~ 8-12)    | Lose a peer; must re-establish  |
| n-k membership change  | Very high (φ ~ 12+) | Trigger threshold reconfiguration |

Low thresholds are cheap to get wrong (retry with another peer). High
thresholds are expensive (removing a peer from the n-k subsystem is
disruptive). The cost of a wrong decision determines the threshold.

### Membership Protocol

Failure detection and membership are related but distinct. Failure detection
asks "is this peer alive right now?" Membership asks "who are the current
members of the system?"

The SWIM protocol (Das, Gupta & Motivala, 2002) combines both:

- **Failure detection** via randomized probing: ping a random peer; if no
  response, ask k other peers to probe it (indirect ping). This avoids false
  positives from transient network issues between specific node pairs.
- **Membership dissemination** via infection-style gossip: membership changes
  (joins, failures) are piggybacked on existing protocol messages, spreading
  epidemically without dedicated multicast.

SWIM scales to large groups (O(1) per-node message load) and tolerates network
asymmetry (indirect probing). Whether SWIM or a simpler protocol is
appropriate depends on the expected number of peers — a system with 5 n-k
nodes doesn't need SWIM's scalability, but a system with 500 edge nodes might.

### What Cannot Be Improved

The following limitations are fundamental, not implementation gaps:

- **No logical-time substitute for wall clocks** in failure detection (FLP).
- **False positives are unavoidable.** A slow peer is indistinguishable from a
  dead one until enough time passes. The phi detector minimizes but cannot
  eliminate them.
- **Statistical sampling is irreducible.** The inter-arrival distribution must
  be estimated from observations. There is no closed-form "correct timeout."
  The phi detector is the most principled way to do this, but it is still
  statistical.

These are not open questions — they are settled results. The system must be
designed to tolerate false positives gracefully (which it does, because CRDTs
don't require accurate failure detection for correctness).

## Deferred Questions

- **Quorum models beyond k-of-n.** Deferred to the federation layer. Within
  a single cluster (same trust domain, simple PKI), uniform k-of-n quorums
  are unnecessary — there is no threshold security requirement. Structured
  quorum models like crumbling walls (Peleg & Wool, 1997) become relevant
  when clusters federate across trust boundaries using Shamir k-of-n (see
  Trust Mechanism section). At that point, the question of whether
  structured quorums compose with threshold security applies to the
  federation layer, not the intra-cluster replication model.

## Settled Decisions (from design sessions)

Decisions made during iterative design conversations. Recorded here so future
sessions don't re-derive them.

### Delta Store Architecture

- **Eager push buffer + anti-entropy fallback.** Two mechanisms, cleanly
  separated. Buffer is in-memory (lost on crash). Anti-entropy from current
  state is the recovery path.
- **One delta per dot.** Each mutator creates one dot, one delta, indexed by
  that dot. Remove deltas (no new dot) are not in the buffer.
- **GC-at-n.** Remove from buffer when all n servers have acknowledged. Not k.
  The buffer is cheap; the correctness argument for k depends on quorum
  intersection guarantees that CRDTs don't provide and don't need.

### Transport

- **TCP for bulk data.** Deltas, state transfer, anything exceeding MTU.
- **UDP for signals.** Discovery, CausalContext summaries, notifications, GC
  state. All under MTU.
- **Zero-ACK by default.** Messages designed so silence is safe. No explicit
  ACK machinery for most messages.
- **Long backoffs for retransmit.** UDP retransmissions use exponential backoff
  with long maximum ceiling. Don't overwhelm networks.

### Identity and Stores

- **Store is the replication unit.** A store is a logical body of data with one
  CausalContext. All participants in a store share the same causal universe.
  Dots within a store are causally ordered; dots across stores are independent.
  Within a store, CRDTs compose via ORMap nesting — "multiple CRDTs" is ORMap
  composition, not multiple independent instances.

- **Full context, partial data.** Every participant in a store replicates the
  full CausalContext (metadata). Data (DotMap contents) is partially replicated
  based on interest. The n-k servers hold full data + full context. Edge/client
  nodes hold partial data + full context. Context growth is bounded by distinct
  replicas (one VV entry per ReplicaID), not by operations or data size.

- **Three-layer identity model.** Network identity (addresses — how to reach a
  node), NodeID (stable global identity across stores), and ReplicaID (dot
  attribution within the algebra). ReplicaID = NodeID — the same string.
  Disambiguation is structural: which store's CausalContext you're looking at,
  not which ReplicaID. A node participating in 3 stores has one NodeID
  appearing in 3 independent CausalContexts.

- **Replica retirement via all-n-ACK.** A VV entry for a permanently gone
  replica can be removed once all n servers confirm they've merged all of that
  replica's dots. Same gate as delta GC — `Missing()` between servers for the
  retired replica's dots must return empty. This bounds VV growth: entries
  don't accumulate forever, they're cleaned up with the same machinery as
  delta buffer eviction. Retirement is a control plane operation triggered
  after high-confidence failure detection and explicit removal from the trust
  group.

### Replication Model

- **No gossip.** Direct anti-entropy between known peers. Servers in the n-k
  subsystem know each other — randomized gossip solves a problem that doesn't
  exist here.
- **Anti-entropy initiation over UDP.** CausalContext summary fits under MTU.
  If `Missing()` returns non-empty, open TCP for delta transfer. Avoids TCP
  overhead when peers are already synced.
- **Broadcast for discovery only.** LAN discovery via UDP broadcast. All other
  communication is unicast to known peers.

### Application-Layer Replication

The replication model supports partial replication generically: full
metadata (CausalContext) replicated to all participants, content
replicated based on application-defined interest. Application-specific
decisions — interest expression, content routing, eviction policy — are
defined by the application layer, not the CRDT library.

For tessera's application-layer replication design (files, blocks,
recipes, content routing, eviction), see
`tessera/docs/plans/REPLICATION-DESIGN.md`.

### Peer Prioritization

- **Peer budget.** Each node declares an explicit peer budget — the number
  of server peers it will maintain — based on its own properties (bandwidth,
  battery, connectivity). A phone on cellular might budget 1 server peer.
  A desktop on a wired LAN might budget all servers. Servers peer with all
  other servers in the n-k subsystem (settled).

- **Phi accrual selects peers.** The peer budget creates slots. The phi
  accrual failure detector — already maintained per-peer — provides the
  selection signal. Fill slots with the most responsive servers (lowest
  latency, highest liveness confidence).

- **Dynamic peer set.** If a peer's phi rises (suspicion of failure), the
  slot opens and the next-best available server fills it. No explicit
  eviction timer — the phi threshold for peer replacement is a policy
  decision, like other phi-based thresholds (see Failure Detection section).

- **Push to all peers in the set.** Eager push to every peer is already
  settled (peerage model). The peer budget limits the fan-out. A client
  with budget 1 pushes to 1 server; server-to-server replication handles
  fan-out within the n-k subsystem.

### Store Membership

- **Server list = AWSet[NodeID] in the store.** The server membership list
  is a CRDT within the store, using the same CausalContext and replication
  path as all other data. Adding or removing a server is a CRDT operation
  that produces a delta. Convergence is guaranteed by the algebra — no
  consensus protocol for the list itself.

- **Authorization gates the operation, not the convergence.** The replication
  layer validates incoming deltas: is this delta's ReplicaID a current
  member of the server list? If yes, merge. If no, drop. The CRDT algebra
  is unchanged — authorization is a filter on the replication path.

- **Any server can modify membership.** Within the same trust domain, any
  current server can add or remove members by operating on the AWSet. If
  the trust model evolves to cross-boundary, Shamir k-of-n can be wired
  in as an authorization filter (see Trust Mechanism section).

- **Bootstrap.** The first server creates the store with itself as the sole
  member, generates its keypair, and adds its `{NodeID, PublicKey}` to the
  server list AWSet.

- **Server removal.** When a server is removed from the AWSet, its future
  deltas are rejected by other servers (no longer a current member). Its
  past deltas remain merged. Replica retirement (all-n-ACK, settled) cleans
  up its VV entry.

- **Client membership is implicit.** Clients do not appear in any membership
  list. A client authenticates (trust mechanism), then anti-entropy from
  empty state provides the initial CausalContext and metadata sync. A
  writing client's existence is implicit in its dots. A read-only client
  is invisible to the server. No join announcement, no leave protocol.

- **Join = anti-entropy from empty.** No special join handshake for data
  sync. A new participant (server or client) starts with empty state.
  `Missing()` on empty CC returns everything. The existing anti-entropy
  mechanism handles initial sync identically to a node reconnecting after
  extended absence. For servers, this includes full data transfer; for
  clients, metadata only (blocks pulled on demand).

- **Leave = phi detects absence.** No explicit leave protocol required.
  Graceful departure can accelerate detection. For servers, high-confidence
  phi triggers membership removal and replica retirement. For clients,
  nothing to clean up unless the client was a writer (replica retirement
  handles its VV entry).

### Trust Mechanism

- **Same trust domain.** Servers are controlled by the same entity. The
  threat model is unauthorized external access, not internal server
  compromise. This makes k-of-n threshold security unnecessary for the
  current design — simple PKI is proportional to the actual threat.

- **Mutual TLS with pinned public keys.** Each server generates an ed25519
  keypair. The server list AWSet holds `{NodeID, PublicKey}` entries.
  Connecting peers authenticate via mutual TLS, verifying the peer's
  certificate against the public key in the CRDT. Standard TLS libraries
  handle the cryptography.

- **Any server can modify membership.** Since servers share a trust domain,
  any current server can add or remove members by operating on the server
  list AWSet. No k-of-n gate required. This is a deliberate simplification
  — if the trust model evolves to span trust boundaries, the `shamir/`
  project provides k-of-n threshold primitives (VSS, proactive refresh)
  that can be wired in as an authorization filter on membership operations.

- **Client authentication via server-issued tokens.** Any server can issue
  a short-lived credential for a client. The client presents this on
  connection. Servers verify against their own issuance or against a
  shared token format. Clients do not need keypairs.

- **Data in transit protected by TLS.** All TCP connections (delta transfer,
  block fetching, state transfer) use TLS. UDP signals (discovery,
  CausalContext summaries, HAVE queries) are not encrypted — they carry
  no sensitive data (hashes, version vectors, presence signals).

- **Shamir remains available.** The `shamir/` project implements Shamir
  secret sharing, Feldman/Pedersen VSS, and proactive share refresh. If
  the trust model evolves to cross-boundary (servers controlled by
  different entities), these primitives gate membership changes: k-of-n
  servers must cooperate to authorize additions, and proactive refresh
  invalidates removed servers' shares. This is a future upgrade, not a
  current requirement.

### Wire Protocol

- **Codec infrastructure exists.** Binary codecs for all dotcontext types
  are implemented in `dotcontext/codec.go`: `CausalContextCodec`,
  `DotCodec`, `DotSetCodec`, `DotFunCodec`, `DotMapCodec`, `MissingCodec`,
  `DeltaBatchCodec[T]`, `CausalCodec[T]`. All use length-prefixed
  little-endian binary encoding with `maxDecodeLen` caps against malformed
  input.

- **Anti-entropy composition exists.** `replication/antientropy.go`
  provides `WriteDeltaBatch` and `ReadDeltaBatch`, composing `Missing()`,
  `DeltaStore.Fetch`, and `DeltaBatchCodec` into a complete
  request-response flow over `io.Reader`/`io.Writer`.

- **Delta payloads are opaque.** The `Codec[T]` interface lets the
  application supply its own serialization for delta values. The
  replication layer encodes framing (dots, contexts, batch counts) and
  delegates payload encoding to the caller. This preserves `crdt`'s
  independence from application types.

- **Message framing.** TCP: `[4-byte big-endian length][1-byte type]
  [payload]`. UDP: `[1-byte type][payload]` (datagram boundary is the
  frame). Message types are partitioned: `0x00–0x7F` reserved for `crdt`
  replication messages, `0x80–0xFF` available for application-defined
  messages (e.g., tessera block transfer).

- **crdt message types:**

  | Type | Code | Transport | Payload codec |
  |------|------|-----------|---------------|
  | DISCOVER | 0x01 | UDP broadcast | NodeID + PublicKey |
  | CC_SUMMARY | 0x02 | UDP | CausalContextCodec |
  | DELTA_BATCH | 0x10 | TCP | DeltaBatchCodec |
  | FULL_STATE | 0x11 | TCP | CausalCodec |
  | PEER_HELLO | 0x12 | TCP | NodeID |

- **TLS wraps TCP.** All TCP connections use mutual TLS (see Trust
  Mechanism section). The PEER_HELLO message is the first application-level
  message after TLS handshake, identifying the NodeID for authorization
  against the server list CRDT.

- **UDP is unencrypted.** UDP signals carry no sensitive data — hashes,
  version vectors, presence. Discovery broadcasts carry NodeID + PublicKey
  for initial peer identification.

## References

### CRDT Foundations

- Almeida, P. S., Shoker, A., & Baquero, C. (2018). Delta state replicated
  data types. *Journal of Parallel and Distributed Computing*, 111, 162-173.
  [arXiv:1603.01529](https://arxiv.org/abs/1603.01529)
  — The foundation paper. Defines dots, causal contexts, dot stores, and the
  join algebra that this codebase implements.

### Ordering and Logical Time

- Lamport, L. (1978). Time, clocks, and the ordering of events in a
  distributed system. *Communications of the ACM*, 21(7), 558-565.
  [ACM](https://dl.acm.org/doi/10.1145/359545.359563)
  — Defines logical clocks (Lamport timestamps) and the happened-before
  relation. Establishes that logical time captures causality but not duration.
  The argument in this document that wall-clock timeouts are irreducible
  (Section: Failure Detection) depends on this distinction: logical clocks
  advance only on events, so silence is invisible to them.

- Charron-Bost, B. (1991). Concerning the size of logical clocks in
  distributed systems. *Information Processing Letters*, 39(1), 11-16.
  [ScienceDirect](https://www.sciencedirect.com/science/article/pii/002001909190055M)
  — Proves the lower bound: tracking causality among n processes requires
  vectors of dimension at least n. Vector clocks of size n are therefore
  optimal — the `CausalContext` version vector meets this bound. Anything
  above O(n) in the causal metadata is redundant overhead. The outlier set
  is the variable cost above this floor; `Compact()` pushes it toward the
  minimum.

### Anti-Entropy and Gossip Protocols

- Demers, A., Greene, D., Hauser, C., Irish, W., Larson, J., Shenker, S.,
  Sturgis, H., Swinehart, D., & Terry, D. (1987). Epidemic algorithms for
  replicated database maintenance. *Proc. 6th ACM Symposium on Principles of
  Distributed Computing (PODC)*, 1-12.
  [ACM](https://dl.acm.org/doi/10.1145/41840.41841)
  — Foundational paper on epidemic (gossip) replication. Introduces the
  anti-entropy protocol and the push/pull/push-pull synchronization taxonomy.
  Two contributions to this design: (1) the anti-entropy mechanism in the
  Peerage section is an instance of their protocol applied to delta-state
  CRDTs, and (2) the push-toward-availability strategy in the Data Plane is
  push-based epidemic replication directed by the availability gradient. Demers
  et al. show that push is more effective for rapid dissemination to all nodes,
  which grounds the "eager push from fragile nodes" policy.

### Impossibility Results

- Fischer, M. J., Lynch, N. A., & Paterson, M. S. (1985). Impossibility of
  distributed consensus with one faulty process. *Journal of the ACM*, 32(2),
  374-382.
  [ACM](https://dl.acm.org/doi/10.1145/3149.214121) |
  [PDF](https://groups.csail.mit.edu/tds/papers/Lynch/jacm85.pdf)
  — Proves that in an asynchronous system, no algorithm can distinguish slow
  from dead. Foundational result that makes wall-clock timeouts irreducible.

### Failure Detection

- Chandra, T. D. & Toueg, S. (1996). Unreliable failure detectors for
  reliable distributed systems. *Journal of the ACM*, 43(2), 225-267.
  [ACM](https://dl.acm.org/doi/10.1145/226643.226647) |
  [PDF](https://www.cs.utexas.edu/~lorenzo/corsi/cs380d/papers/p225-chandra.pdf)
  — Formalizes failure detector classes by completeness and accuracy
  properties. Shows that even unreliable detectors (that make infinite
  mistakes) suffice for consensus. Defines the theoretical framework that
  concrete detectors like phi accrual implement.

- Hayashibara, N., Defago, X., Yared, R., & Katayama, T. (2004). The phi
  accrual failure detector. *Proc. 23rd IEEE International Symposium on
  Reliable Distributed Systems (SRDS)*, 66-78.
  [IEEE](https://www.computer.org/csdl/proceedings-article/srds/2004/22390066/12OmNvT2phv) |
  [ResearchGate](https://www.researchgate.net/publication/29682135_The_ph_accrual_failure_detector)
  — Concrete method. Outputs continuous suspicion level instead of binary
  alive/dead. Adapts to per-peer network conditions. Used in Cassandra and
  Akka. The recommended failure detection approach for this system.

- van Renesse, R., Minsky, Y., & Hayden, M. (1998). A gossip-style failure
  detection service. *Middleware'98*, Springer, London.
  [Springer](https://link.springer.com/chapter/10.1007/978-1-4471-1283-9_4) |
  [PDF](https://www.cs.cornell.edu/home/rvr/papers/GossipFD.pdf)
  — Gossip-based failure detection that scales well and leverages network
  topology for efficiency. Combines gossip with broadcast for partition
  detection. Relevant to peer discovery and liveness in large groups.

### Membership Protocols

- Das, A., Gupta, I., & Motivala, A. (2002). SWIM: scalable weakly-consistent
  infection-style process group membership protocol. *Proc. International
  Conference on Dependable Systems and Networks (DSN)*, 303-312.
  [IEEE](https://ieeexplore.ieee.org/document/1028914/) |
  [PDF](https://www.cs.cornell.edu/projects/Quicksilver/public_pdfs/SWIM.pdf)
  — Concrete method. Separates failure detection (randomized probing with
  indirect ping) from membership dissemination (epidemic piggybacking).
  O(1) per-node message load. Production-proven at Uber (Ringpop). Candidate
  protocol for large peer groups in this system.

### Threshold Cryptography

- Shamir, A. (1979). How to share a secret. *Communications of the ACM*,
  22(11), 612-613.
  [ACM](https://dl.acm.org/doi/10.1145/359168.359176)
  — Concrete method. k-of-n secret sharing over a finite field: any k shares
  reconstruct the secret, fewer than k reveal nothing. The distributed trust
  authority in the Control Plane uses this as its cryptographic primitive —
  k-of-n servers must cooperate to issue credentials. Implemented in the
  companion `shamir/` project in this workspace.

### Consensus Protocols (Contrast)

- Lamport, L. (1998). The part-time parliament. *ACM Transactions on Computer
  Systems*, 16(2), 133-169.
  [ACM](https://dl.acm.org/doi/10.1145/279227.279229)
  — Defines the Paxos consensus algorithm: quorum-for-permission, where
  below-quorum means hard stop (no progress). This design explicitly rejects
  this model. CRDTs guarantee convergence without consensus, so quorum here is
  for confirmation (a weaker guarantee layered on top), not for permission.
  The measurable difference: Paxos blocks writes when fewer than a majority
  are reachable; this system continues writing locally with degraded durability
  confirmation.

- Ongaro, D. & Ousterhout, J. (2014). In search of an understandable
  consensus algorithm. *Proc. USENIX Annual Technical Conference (ATC)*,
  305-319.
  [USENIX](https://www.usenix.org/conference/atc14/technical-sessions/presentation/ongaro)
  — Raft consensus algorithm. Same quorum-for-permission model as Paxos with
  different mechanics (leader election, log replication). Named alongside Paxos
  in Section "No Quorum for Writes" as the class of systems from which this
  design departs: below-quorum in Raft = hard stop; here = reduced guarantees,
  never system failure.

### Quorum Systems

- Peleg, D. & Wool, A. (1997). Crumbling walls: a class of practical and
  efficient quorum systems. *Distributed Computing*, 10, 87-97.
  [Springer](https://link.springer.com/article/10.1007/s004460050027)
  — Structured quorum system where nodes are arranged in rows of varying
  widths. Quorum = one full row + one representative from each row below.
  Availability increases and load decreases with system size. Potential
  alternative to uniform k-of-n for data quorums; compatibility with
  threshold security is an open question.

- Peleg, D. & Wool, A. (1998). The availability of crumbling wall quorum
  systems. *Discrete Applied Mathematics*, 83(1-3), 213-228.
  [ScienceDirect](https://www.sciencedirect.com/science/article/pii/S0166218X96000169)
  — Companion paper analyzing availability properties of crumbling walls.
