build:
	go build
	# GOOS="js" GOARCH="wasm" go build -o client/client.wasm client/client.go
