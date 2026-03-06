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

Data flows toward higher availability, like gravity. Each node pushes deltas
toward the most available peers it knows about. This is not a heuristic — it
is the core strategy:

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

This is fundamentally different from consensus-based systems (Raft, Paxos)
where below-quorum means hard stop. Here, fewer peers means lower (or
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
the missing deltas. This is the anti-entropy protocol from Almeida et al. 2018,
and the machinery exists in `dotcontext/`.

## Discovery

A node must find at least one peer before peerage is useful. Three mechanisms,
which coexist:

| Mechanism           | Range               | Configuration              |
|---------------------|---------------------|----------------------------|
| Broadcast           | Local network (LAN)  | None                       |
| Configured address  | Global (internet)    | One address required       |
| Introduction        | Grows from above     | None beyond initial peer   |

**Broadcast** — zero-configuration discovery on the local network (mDNS,
multicast). A phone discovers a laptop on the same wifi. Bounded by network
segment; does not cross the internet. Can cause broadcast storms, which limits
reach.

**Configured address** — a node is told the address of a peer (typically a
server). Works globally. Requires out-of-band knowledge — someone must provide
the address.

**Introduction** — if A knows B, and B knows C, B can introduce A to C. This
is gossip-based discovery. It requires one bootstrap connection; from there,
the peer set grows organically. Introductions are authenticated: the
introducing peer vouches for the new peer using credentials from the control
plane.

A server is a natural bootstrap point: always available, stable address,
discoverable. A client configures one server address, connects, and learns
about other peers through introductions.

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
distributed across multiple servers using threshold operations:

- k-of-n servers must participate to issue credentials
- No single server's failure breaks the trust chain

### Shared Infrastructure

The k-of-n server coordination required for trust operations is the same
infrastructure that supports quorum-confirmed durability on the data plane.
Both planes share the same set of coordinating servers and the same threshold
property. This is not a coincidence — both problems (trust and durability)
require the same thing: confirmation from multiple high-availability nodes.

Since security demands k-of-n coordination regardless, the incremental cost
of data durability confirmation is low. The system pays the quorum tax once
and uses it for both planes.

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

## Open Questions

- **Partial replication.** Does every node store every CRDT, or can nodes hold
  subsets? Full replication is bounded by the smallest node's capacity. Partial
  replication requires defining which subsets and adds routing complexity.

- **Downward push.** When a server receives a delta from a client, does it push
  to other clients proactively, or do clients pull on reconnect? Eager downward
  push maximizes speed but may push data to nodes that don't need it.

- **Peer prioritization.** When a node has multiple peers, which gets the delta
  first? Highest availability? Lowest latency? Round-robin?

- **Wire protocol.** What does the delta exchange protocol look like on the
  wire? Context comparison, delta encoding, authentication handshake.

- **CRDT identity vs. node identity.** What is the relationship between
  `ReplicaID` in `dotcontext/` and the node's network identity?

- **Multi-CRDT coordination.** How do multiple CRDT instances relate? Does a
  node subscribe to specific CRDTs, or replicate all of them?

- **Trust mechanism.** The model requires authenticated peering and distributed
  trust authority. The specific cryptographic mechanism (shared secrets, PKI,
  threshold signatures) is deferred.
