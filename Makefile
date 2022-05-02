
run: build
	./temp/twofactor

build:	tidy wasm binary

tidy:
	# @go mod tidy

wasm:
	rm -rf server/web/app.wasm
	GOARCH=wasm GOOS=js go build -o server/web/app.wasm .

binary:
	@mkdir -p temp
	@go build -o temp/twofactor .




