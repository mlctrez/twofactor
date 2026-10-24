# AGENTS.md

Repo-specific rules for agents and contributors working on twofactor.

## Build and test

```sh
CGO_ENABLED=0 go vet ./...
CGO_ENABLED=0 go test ./...
make build   # also confirms the GOOS=js GOARCH=wasm UI build compiles
```

`make build` writes `server/web/app.wasm` and `temp/twofactor`; both are
gitignored, so don't commit them.

## Deploy

The repository owner deploys twofactor. Agents never deploy it (don't run `make deploy`);
after a PR is merged and tagged, just report that.
