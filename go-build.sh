#!/bin/bash

set -e

BINARY_PATH=${BINARY_PATH:-$(pwd)}

go mod download

go mod verify

go get -d github.com/rakyll/statik

GOPATH=$(go env GOPATH)

$GOPATH/bin/statik -src=img

if [[ "${TARGETARCH}" == "arm64" ]]; then
  CC="aarch64-linux-gnu-gcc" go build -o "${BINARY_PATH}" ./cmd/main.go
else
  go build -o "${BINARY_PATH}" ./cmd/main.go
fi

cp -v main pibox-framebuffer
