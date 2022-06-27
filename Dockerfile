FROM --platform=$BUILDPLATFORM golang:1.17-bullseye AS build
ARG TARGETARCH
ARG BUILD_VERSION
ENV BINARY_PATH="/pibox-framebuffer-linux-${TARGETARCH}-v${BUILD_VERSION}"

ENV APP_HOME /go/src/pibox-framebuffer
WORKDIR "$APP_HOME"

RUN apt-get -yqq update && apt-get -yqq install gcc build-essential gcc-aarch64-linux-gnu

COPY . .

ENV CGO_ENABLED=1
ENV GOOS=linux
ENV GOARCH="${TARGETARCH}"

RUN ./go-build.sh

FROM scratch
ARG TARGETARCH
ARG BUILD_VERSION
ENV BINARY_PATH="/pibox-framebuffer-linux-${TARGETARCH}-v${BUILD_VERSION}"

COPY --from=build "${BINARY_PATH}" /
