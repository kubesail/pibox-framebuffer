FROM --platform=$BUILDPLATFORM golang:1.18-bullseye AS build
ARG BUILDARCH
ARG BUILD_VERSION
ENV BINARY_PATH="/pibox-framebuffer-${BUILDARCH}-v${BUILD_VERSION}"

ENV APP_HOME /go/src/pibox-framebuffer
WORKDIR "$APP_HOME"

COPY . .

RUN go mod download
RUN go mod verify
RUN go build -o "${BINARY_PATH}"

FROM scratch
ARG BUILDARCH
ARG BUILD_VERSION
ENV BINARY_PATH="/pibox-framebuffer-${BUILDARCH}-v${BUILD_VERSION}"

COPY --from=build "${BINARY_PATH}" /
