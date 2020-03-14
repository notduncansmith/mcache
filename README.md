# Manifest Cache - Edge data replication made easy

Manifest Cache (or MCache) is a hybrid on-disk/in-memory document store written in Go. It offers an HTTP API and JS client library, and is designed to serve end-users by replicating their server-stored data directly to their client devices. The primary intended usage pattern involves users initially downloading their entire personal dataset and storing it in local storage, then sub. For applications with data siloed by user or group of users, this strategy can save massive amounts of bandwidth and latency.

## How it works

MCache stores **documents**, which are opaque blobs decorated with `id` and `updatedAt` properties of types `string` and `int64` respectively, in independent collections called **indexes**. Special documents in an index called **manifests** hold lists of other document IDs. A manifest may contain references to any other documents within the same index.

Whenever a change is made on the primary data store, this change must be reflected to MCache through a GraphQL mutation. Updates to multiple documents in the same index may be batched in a single request. Documents must be provided in full (because MCache is unaware of the encoding/structure of document bodies, it cannot merge deltas).

A query to MCache for the latest changes includes a manifest ID and the timestamp of the client's most recently-updated document. That query will return any documents in the manifest whose MCache entries have been updated since that timestamp. Documents are always delivered in full.

The client library stores documents received from queries in `localStorage`. It provides facilities for applications to query this data and call hooks when updates are received.

## Full Docs

TODO

## License

Released under [The MIT License](https://opensource.org/licenses/MIT) (see `LICENSE.txt`).

Copyright 2020 Duncan Smith