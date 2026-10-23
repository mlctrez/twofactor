# twofactor

A small TOTP authenticator: a Go WebAssembly web app served by a Go server on port `9000`.

## Usage

- Entries are kept in the browser. Click the version text to open edit mode, where you can
  add, rename and delete entries, and upload or download the server copy (`storage.json`).
- `make run` builds and runs it locally on port `8080`. `make deploy` builds it and installs it on the deploy host named in the `Makefile`.

## API

- `GET /api/labels` lists entry labels (`Issuer Name`).
- `GET /api/code?label=<label>` returns the current code for one entry.

```shell
curl -s --get http://<host>:9000/api/code --data-urlencode 'label=Example alice' | jq -r .code
```

## Security

This app has no authentication or TLS. Anyone who can reach it can read every code, and
`/api/storage` exposes the raw secrets. It must be secured some other way, such as firewall
rules or Tailscale ACLs that limit who can reach the port.
