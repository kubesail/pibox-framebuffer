#!/bin/bash

BINARY_PATH=${BINARY_PATH:-$(pwd)}

go mod download

go mod verify

if [[ "${TARGETARCH}" == "arm64" ]]; then
  CC="aarch64-linux-gnu-gcc" go build -o "${BINARY_PATH}" ./cmd/main.go
else
  go build -o "${BINARY_PATH}" ./cmd/main.go
fi
