#!/bin/bash

TAG="kubesail/pibox-framebuffer:v$(cat VERSION.txt)"

# Enable docker experimental mode:
#  - echo '{ "experimental": true }' > /etc/docker/daemon.json
#  - echo '{ "experimental": "enabled" }' > ~/.docker/config.json
# install buildx (https://github.com/docker/buildx/releases)

# docker buildx create --name mybuilder
# docker buildx use mybuilder
# docker buildx inspect --bootstrap
# docker run --privileged --rm tonistiigi/binfmt --install all

# Troubleshooting:
# buildx stop... buildx inspect --bootstrap

BUILDX_PREFIX="docker buildx"
command -v buildx > /dev/null && BUILDX_PREFIX="buildx"

mkdir -p output

DOCKER_BUILDKIT=1 ${BUILDX_PREFIX} build \
  --pull \
  --platform "linux/amd64,linux/arm64" \
  --build-arg BUILD_VERSION="$(cat VERSION.txt)" \
  -t ${TAG} \
  -o output/ .
