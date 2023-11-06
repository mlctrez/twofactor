
VERSION=$(shell git describe --abbrev=0 --tags 2>/dev/null || echo "0.0.0")
COMMIT=$(shell git rev-parse --short HEAD || echo "HEAD")

run: build
	PORT=8080 DEV=true ./temp/twofactor

build:	tidy wasm binary

tidy:
	# @go mod tidy

wasm:
	@rm -rf server/web/app.wasm
	@GOARCH=wasm GOOS=js go build -o server/web/app.wasm \
		-ldflags "-w -X $(shell go list).Version=$(VERSION) -X $(shell go list).Commit=$(COMMIT)" \
		cmd/twofactor/twofactor.go

binary: wasm
	@mkdir -p temp
	@CGO_ENABLED=0  go build -o temp/twofactor	\
		-ldflags "-w -X $(shell go list).Version=$(VERSION) -X $(shell go list).Commit=$(COMMIT)" \
		cmd/twofactor/twofactor.go

deploy: binary
	scp temp/twofactor goservice:/tmp
	ssh goservice sudo /tmp/twofactor -action deploy




