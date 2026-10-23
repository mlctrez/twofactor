# twofactor

[![Go Report Card](https://badge.mlctrez.com/mlctrez/twofactor)](https://goreportcard.com/report/github.com/mlctrez/twofactor)

created by [tigwen](https://github.com/mlctrez/tigwen)

## API

The server keeps a copy of the browser storage in `storage.json` (`GET`/`POST /api/storage`).
It also exposes read-only endpoints that compute TOTP codes server-side, so callers never
need the raw secrets:

| Endpoint | Response |
| --- | --- |
| `GET /api/labels` | `200 {"labels": ["Example alice", ...]}` |
| `GET /api/code?label=<label>` | `200 {"label": "Example alice", "code": "123456", "remaining": 17, "period": 30}` |

`label` is the entry name as shown in the UI (`Issuer Name`, or just `Name` when there is no
issuer or the name already starts with the issuer). An exact match is tried first, then a
case-insensitive match. `remaining` is the number of seconds left in the current period.

Errors are JSON `{"error": "..."}`: `400` when `label` is missing or the entry is HOTP,
`404` when no entry matches, `409` when the label matches more than one entry (the response
also lists the `matches`), and `500` when `storage.json` is missing or unreadable.

```shell
curl -s 'http://dune:9000/api/labels'
curl -s 'http://dune:9000/api/code?label=Example%20alice'
curl -s --get 'http://dune:9000/api/code' --data-urlencode 'label=Example alice' | jq -r .code
```

Anyone who can reach the server can read codes (and `/api/storage` already exposes the raw
secrets), so only run it on a trusted network.
