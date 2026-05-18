# MaxIO Roadmap

MaxIO is currently a runnable prototype with a library-first runtime, HTTP API,
single-shard Dragonboat metadata, erasure-coded local storage, object indexing,
event publishing, Web UI, and a Raft-backed background repair scheduler.

This document tracks the remaining work required before the project can be
treated as a production-grade object storage service.

## Current status

- The application is library-first and can be embedded through the root Go package.
- Runtime composition is dix-first, with config, logging, event bus, HTTP, Raft,
  metadata, storage, index, S3 endpoint registration, scheduler, and repair
  assembled as modules.
- Metadata is backed by the Raft state machine rather than an external KV store.
- Object data is written through the current storage engine with local erasure
  coding and repair primitives.
- Storage placement now has local and remote `StorageNode` implementations. The
  remote shard path uses the internal HTTP shard endpoint, and tests cover a
  two-node write/read loop through that transport.
- Background repair is scheduled through gocron and guarded by Raft leadership,
  so only the current leader runs cluster-wide repair jobs.
- Background repair now scrubs healthy objects by verifying shard checksums and
  decoded object checksums, and exposes scrub counters in repair status.
- Object-level dedupe has a leader-only background scanner that reconciles
  committed object hashes, blob reference counts, orphan blob refs, and object
  layouts against canonical blob refs.
- Cluster backend now exposes a normalized node registry that merges Raft
  membership, gossip discovery state, and storage node registration/liveness.
- Cluster node registry, metrics, and rebalance/decommission APIs now expose
  current object, shard, and used byte ownership derived from committed shard
  layouts. Rebalance plan/action now validates Raft membership before scanning
  or moving object layouts, replacement reports the old node's logical bytes, and
  replacement validation maps local/missing replicas to explicit client errors.
- Basic S3-compatible HTTP endpoints exist, but S3 compatibility is not yet a
  production target.

## Production gaps

### 1. Distributed write path

The biggest missing piece is the true distributed object write path. Object data
still needs to flow through node placement, shard persistence, quorum or commit
rules, metadata commit, index update, and event publication as one recoverable
pipeline.

Required work:

- Add a `StorageNode` abstraction for local and remote shard targets.
- Add a `PlacementPlanner` that chooses shard targets based on membership,
  capacity, node health, and failure domains.
- Implement a distributed write pipeline that writes data shards and parity
  shards to selected nodes before metadata is committed.
- Make object reads resolve shard locations from metadata and reconstruct from
  remote shards when needed.
- Define failure semantics for partial writes, pending metadata, and retries.

### 2. Write consistency and recovery

The system needs a clear object lifecycle so crashes and partial failures can be
recovered deterministically.

Required work:

- Define states such as pending, committed, deleted, and tombstoned.
- Persist enough write-intent data to recover after process or node crashes.
- Add orphan shard garbage collection.
- Add pending object expiration and retry policy.
- Ensure index and event publication follow committed metadata, not partial data.

### 3. Data verification and self-healing

Repair exists, but production repair needs stronger verification and operational
controls.

Required work:

- Store and verify per-shard checksums.
- Add object-level checksum validation.
- Add background scrub jobs for bitrot detection.
- Add repair rate limiting and retry backoff.
- Expose repair progress, failures, and last-run status.
- Support repair from remote healthy replicas or shards.

### 4. Cluster membership lifecycle

Membership exists at the Raft and discovery layer, but production clusters need
safe operational flows.

Required work:

- Implement join and bootstrap flows that are safe for repeated startup.
- Add node drain, decommission, and rebalance flows.
- Track node liveness, disk capacity, and shard ownership.
- Handle node loss, node replacement, and address changes.
- Add admin APIs and Web UI screens for membership operations.

### 5. Replication and erasure placement

Erasure coding exists locally, but the placement model must be cluster-aware.

Required work:

- Define storage classes such as replicated and erasure-coded.
- Place chunks across distinct nodes and, later, distinct failure domains.
- Prevent unsafe layouts where too many chunks land on one node or disk.
- Add rebalancing when nodes are added or removed.
- Add read repair when stale or missing shards are detected during reads.

### 6. S3 compatibility

The S3 layer should remain a compatibility layer over the MaxIO core API. It is
not ready for production yet.

Required work:

- Add AWS Signature V4 authentication.
- Add access keys, secret keys, and request authorization.
- Implement multipart upload.
- Implement presigned URLs.
- Implement range reads and correct ETag semantics.
- Align XML error responses and status codes with S3 behavior.
- Add S3 compatibility tests.

### 7. Security and access control

The service should not be exposed outside a trusted development environment
until security primitives are implemented.

Required work:

- Add admin authentication.
- Add access key and secret management.
- Add bucket/object authorization model.
- Support TLS configuration.
- Add audit logs for data and admin operations.
- Protect internal cluster APIs.

### 8. Observability

Production operation requires clear visibility into cluster and storage behavior.

Required work:

- Add metrics for request latency, throughput, error rates, and object sizes.
- Add metrics for Raft state, leader changes, membership, and apply latency.
- Add metrics for storage capacity, shard health, repair, and scrub.
- Add tracing around write/read/delete/search paths.
- Add structured audit logs.
- Expose health and readiness checks suitable for orchestration.

### 9. Testing and failure injection

The current test coverage is not enough to validate object storage correctness.

Required work:

- Add integration tests for single-node and multi-node clusters.
- Add crash recovery tests for partial writes and pending metadata.
- Add network partition and leader-change tests.
- Add corrupted shard and missing shard tests.
- Add concurrent put/get/delete/list tests.
- Add S3 compatibility tests.
- Add long-running soak tests.

### 10. Packaging and operations

The project needs production deployment assets and upgrade guidance.

Required work:

- Add Docker image build.
- Add example config files.
- Add systemd and container deployment examples.
- Add Kubernetes examples after the cluster model stabilizes.
- Document data directory layout.
- Document backup, restore, and upgrade procedures.

## Recommended implementation order

### Phase 1: Distributed storage core

- Add `StorageNode` and `PlacementPlanner`.
- Implement local node through the same abstraction used by future remote nodes.
- Persist shard placement metadata.
- Update object read/write paths to use placement metadata.
- Add integration tests for object write/read over planned shard layouts.

### Phase 2: Recoverable writes

- Add object lifecycle states.
- Add write-intent persistence.
- Add orphan shard garbage collection.
- Add pending object recovery on startup.
- Make index and events strictly follow committed metadata.

### Phase 3: Cluster-aware data durability

- Add remote shard write/read transport.
- Distribute data and parity shards across multiple nodes.
- Add read reconstruction from remote shards.
- Add read repair.
- Extend repair scheduler to repair from remote healthy shards.

### Phase 4: Cluster operations

- Harden bootstrap and join flows.
- Add node drain and decommission.
- Add rebalancing.
- Add admin APIs and Web UI views for cluster operations.

### Phase 5: Compatibility, security, and operations

- Complete S3 SigV4 and multipart upload.
- Add access control and admin authentication.
- Add metrics, tracing, and audit logs.
- Add deployment assets and operational documentation.

## Immediate next step

The next implementation should continue turning the prototype into a production
storage service in small, testable iterations.

1. Repair hardening

- Store and verify per-shard checksums.
- Add object-level checksum validation.
- Add background scrub jobs for bitrot detection.
- Add repair rate limiting and retry backoff.
- Expose repair progress, failures, and last-run status.
- Repair missing or corrupted shards from remote healthy shards.

2. Cluster lifecycle

- Make bootstrap and join flows idempotent across restarts.
- Add drain, decommission, and rebalance flows.
- Track node liveness, disk capacity, and shard ownership.
- Handle node loss, node replacement, and address changes.
- Add admin APIs and Web UI screens for cluster membership operations.

3. S3 compatibility

- Add AWS Signature V4 authentication.
- Add access keys, secret keys, and request authorization.
- Implement multipart upload.
- Implement presigned URLs.
- Implement range reads and correct ETag semantics.
- Align XML error responses and status codes with S3 behavior.
- Add S3 compatibility tests.

4. Security

- Add admin authentication.
- Add access key and secret management.
- Add bucket/object authorization.
- Support TLS configuration.
- Add audit logs for data and admin operations.
- Protect internal cluster and shard APIs.

5. Observability

- Add metrics for request latency, throughput, error rates, and object sizes.
- Add Raft metrics for state, leader changes, membership, and apply latency.
- Add storage metrics for capacity, shard health, repair, and scrub.
- Add tracing around write/read/delete/search paths.
- Add structured audit logs.
- Expose health and readiness checks suitable for orchestration.

6. Test coverage

- Add multi-node integration tests beyond the current remote shard transport
  loop.
- Add leader-change, network-partition, and node-restart tests.
- Add partial write and crash recovery tests.
- Add corrupted shard and missing shard tests.
- Add concurrent put/get/delete/list tests.
- Add S3 compatibility tests.

7. Operations

- Add Docker image build.
- Add example configuration files.
- Add systemd and container deployment examples.
- Add Kubernetes examples after the cluster model stabilizes.
- Document data directory layout.
- Document backup, restore, and upgrade procedures.
