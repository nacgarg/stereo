build:
	go build
	cd stereo-client && npm run build
	
	# GOOS="js" GOARCH="wasm" go build -o client/client.wasm client/client.go
