
run: build
	DEV=true ./temp/twofactor

build:	tidy wasm binary

tidy:
	# @go mod tidy

wasm:
	rm -rf server/web/app.wasm
	GOARCH=wasm GOOS=js go build -o server/web/app.wasm .

binary:
	@mkdir -p temp
	@go build -o temp/twofactor .

deploy: binary
	scp temp/twofactor goservice:/tmp
	ssh goservice sudo /tmp/twofactor -action deploy




