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

## Container image

Build the local image:

```sh
docker build -t maxio:dev .
```

Run a single-node development container with a persistent data volume:

```sh
docker run --rm \
  --name maxio \
  -p 8080:8080 \
  -p 63000:63000 \
  -p 7946:7946 \
  -v maxio-data:/app/data \
  -e MAXIO_ADMIN_TOKEN="$MAXIO_ADMIN_TOKEN" \
  maxio:dev
```

The image copies `config.example.json` to `/app/config.json`. Override config
values with environment variables such as `MAXIO_ADMIN_TOKEN`,
`MAXIO_API_TOKEN`, `MAXIO_RAFT_ADDRESS`, and `MAXIO_STORAGE_ADDRESS`.

## Admin protection

Set `admin_token` or `MAXIO_ADMIN_TOKEN` to protect management and internal shard APIs.
Set `api_token` or `MAXIO_API_TOKEN` to protect bucket and object APIs.
Set `s3_access_key`, `s3_secret_key`, and `s3_region` to require SigV4 header or presigned URL authentication for S3-compatible APIs.
Set `http_body_limit` to control the maximum request body accepted by the Fiber HTTP adapter. The default is `1073741824` bytes so standard S3 multipart upload parts work out of the box.

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

If both S3 key fields are empty, S3-compatible APIs run without authentication for local development. If either S3 key field is configured, both must be configured and S3 clients must send `Authorization: AWS4-HMAC-SHA256 ...` plus `X-Amz-Date`, or use standard presigned URL query parameters.

S3 multipart upload is supported through the compatibility path. In-progress upload state is staged under `data_dir/s3-multipart` and completed objects are committed through the normal MaxIO object write path.

`/healthz` and `/readyz` remain unauthenticated for load balancers.

## TLS termination

Terminate TLS at a reverse proxy, ingress, or load balancer in front of MaxIO.

The current httpx runtime used by MaxIO exposes plain HTTP serving. Keep MaxIO on a private network and expose only the TLS terminator publicly.

Example topology:

```text
client -> TLS proxy or ingress -> MaxIO HTTP address
```

Forward these headers unchanged when admin or API tokens are used:

```text
Authorization
X-Maxio-Control
X-Maxio-API
```

The internal shard API `/_internal/storage/shards/*` must only be reachable by trusted MaxIO nodes or by a private service network. If `admin_token` is configured, MaxIO remote shard transport automatically sends `X-Maxio-Control`.

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

## Index operations

Inspect index worker status:

```sh
curl -H "Authorization: Bearer $MAXIO_ADMIN_TOKEN" http://127.0.0.1:8080/_index/status
```

Rebuild the derived Bleve index from committed object metadata and object content:

```sh
curl -X POST -H "Authorization: Bearer $MAXIO_ADMIN_TOKEN" http://127.0.0.1:8080/_index/rebuild
```
