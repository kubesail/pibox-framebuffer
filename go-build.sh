#!/bin/bash

go mod download

go mod verify

if [[ "${TARGETARCH}" == "arm64" ]]; then
  CC="aarch64-linux-gnu-gcc" go build -o "${BINARY_PATH}"
else
  go build -o "${BINARY_PATH}"
fi
