# Manifest Cache - Fast offline-first apps

Manifest Cache (or MCache) is a disk-backed in-memory document store written in Go. It aims to support offline-first applications and improve both bandwidth consumption and latency for multi-tenant applications that serve siloed data over HTTP.

It is designed to directly serve end-user HTTP requests, replicating their server-stored data to their client devices. The primary intended usage pattern is users initially downloading their personal working set and storing it locally, then subsequently querying for updates. For applications with largely or entirely siloed datasets, this distribution strategy can save massive amounts of bandwidth and latency compared to traditional methods.

## How it works

MCache stores **documents**, which are opaque blobs decorated with `ID` and `UpdatedAt` properties, in independent collections called **indexes**. Special documents in an index called **manifests** hold lists of other document IDs.

Indexes are created with an HTTP POST request, and documents (including manifests) are updated with HTTP PUT requests. Updates to multiple documents in the same index may be batched in a single request. Documents must be provided in full; MCache is unaware of the encoding structure of document bodies and cannot merge document bodies.

A query to MCache includes an index ID, a manifest ID, and a timestamp. MCache will respond with any documents in the manifest that have been updated since the given timestamp. Documents are always delivered in full. Documents cannot be deleted, but they can be updated with a `Deleted` property and an empty `Body`. Indexes cannot be deleted via the API, but since each index is contained in a single standalone file on disk, index files can be deleted while the server is not running.

## HTTP API

### `POST /i/:indexID`

_Create Index_

- **Body:** Empty
- **Response:** New Index

```
$ curl -X POST 'http://localhost:1337/i/example'
{"id":"example"}
```

### `PUT /i/:indexID`

_Update Indexed Documents_

- **Body:** JSON-encoded array of Document objects (UpdatedAt on given Documents is ignored since this property is set automatically on write)
- **Response:** JSON-encoded DocSet object containing updated Documents

```
$ curl -X PUT -d '[{"id": "a", "body": "RG9jdW1lbnQgQQ==", "deleted": false }, { "id": "m", "body": "eyJhIjp7fX0=", "deleted": false }]' 'http://localhost:1337/i/example'
{
  "docs": {
    "a": {
      "id": "a",
      "updatedAt": 1609096924,
      "body": "RG9jdW1lbnQgQQ==",
      "deleted": false
    },
    "m": {
      "id": "m",
      "updatedAt": 1609096924,
      "body": "eyJhIjp7fX0=",
      "deleted": false
    }
  },
  "start": 1609096924,
  "end": 1609096924
}
```

### `GET /i/:indexID/m/:manifestID/@/:updatedAfter`

_Query Indexed Documents_

- **Response:** JSON-encoded DocSet object containing Documents that satisfy the query

```
$ curl 'http://localhost:1337/i/example/m/m/@/0'
{
  "docs": {
    "a": {
      "id": "a",
      "updatedAt": 1609096924,
      "body": "RG9jdW1lbnQgQQ==",
      "deleted": false
    },
    "m": {
      "id": "m",
      "updatedAt": 1609096924,
      "body": "eyJhIjp7fX0=",
      "deleted": false
    }
  },
  "start": 1609096924,
  "end": 1609096924
}
```

## License

Released under [The MIT License](https://opensource.org/licenses/MIT) (see `LICENSE.txt`).

Copyright 2020 Duncan Smith
