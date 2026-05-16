# MaxIO deployment notes

## Single node

Use `config.example.json` as the baseline config. For local single-node runs, keep:

```json
{
  "raft_node_id": 1,
  "raft_bootstrap": true,
  "raft_join": false,
  "raft_initial_members": ""
}
```

Start the server with the default config path:

```sh
./maxio
```

## Admin protection

Set `admin_token` or `MAXIO_ADMIN_TOKEN` to protect management and internal shard APIs.
Set `api_token` or `MAXIO_API_TOKEN` to protect bucket and object APIs.

Authenticated requests can use either header:

```sh
Authorization: Bearer <token>
X-Maxio-Control: <token>
```

Protected paths include:

```text
/_cluster/*
/_repair/*
/_internal/*
/_search
/metrics
```

When `api_token` is configured, bucket and object routes also require either:

```sh
Authorization: Bearer <api-token>
X-Maxio-API: <api-token>
```

The admin token is also accepted for bucket and object routes.

`/healthz` and `/readyz` remain unauthenticated for load balancers.

## Multi-node bootstrap

Each node needs a stable raft address and an HTTP storage address.

Example initial members:

```text
1=10.0.0.1:63000,2=10.0.0.2:63000,3=10.0.0.3:63000
```

Node 1:

```json
{
  "raft_node_id": 1,
  "raft_address": "10.0.0.1:63000",
  "storage_address": "10.0.0.1:8080",
  "raft_bootstrap": true,
  "raft_initial_members": "1=10.0.0.1:63000,2=10.0.0.2:63000,3=10.0.0.3:63000"
}
```

Node 2 and node 3 use their own `raft_node_id`, `raft_address`, and `storage_address`, with the same `raft_initial_members`.

## Runtime cluster operations

List raft members:

```sh
curl -H "Authorization: Bearer $MAXIO_ADMIN_TOKEN" http://127.0.0.1:8080/_cluster/members
```

Synchronize storage node HTTP addresses from raft and gossip discovery:

```sh
curl -X POST -H "Authorization: Bearer $MAXIO_ADMIN_TOKEN" http://127.0.0.1:8080/_cluster/storage-nodes/sync
```

Drain a replica from new shard placements:

```sh
curl -X POST -H "Authorization: Bearer $MAXIO_ADMIN_TOKEN" http://127.0.0.1:8080/_cluster/members/2/drain
```

Preview remaining shard references:

```sh
curl -H "Authorization: Bearer $MAXIO_ADMIN_TOKEN" "http://127.0.0.1:8080/_cluster/rebalance/plan?replica_id=2"
```

Rebalance shards away from a drained replica:

```sh
curl -X POST -H "Authorization: Bearer $MAXIO_ADMIN_TOKEN" "http://127.0.0.1:8080/_cluster/rebalance?replica_id=2"
```

Remove the replica after rebalance:

```sh
curl -X DELETE -H "Authorization: Bearer $MAXIO_ADMIN_TOKEN" http://127.0.0.1:8080/_cluster/members/2
```

The remove operation is guarded. It returns conflict if object metadata still references the target replica.

## Node replacement

The replacement endpoint adds the new replica, syncs storage nodes, drains and rebalances the old replica, then removes the old replica if safe:

```sh
curl -X POST \
  -H "Authorization: Bearer $MAXIO_ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"replica_id":4,"target":"10.0.0.4:63000"}' \
  http://127.0.0.1:8080/_cluster/members/2/replace
```

## Observability

Health:

```sh
curl http://127.0.0.1:8080/healthz
```

Readiness:

```sh
curl http://127.0.0.1:8080/readyz
```

Metrics:

```sh
curl -H "Authorization: Bearer $MAXIO_ADMIN_TOKEN" http://127.0.0.1:8080/metrics
```
