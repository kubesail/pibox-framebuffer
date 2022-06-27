FROM golang:1.18-bullseye as builder-stage

ENV APP_HOME /go/src/pibox-framebuffer
WORKDIR "$APP_HOME"

COPY . .

# RUN go mod download
# RUN go mod verify
RUN go build -o /pibox-framebuffer

FROM scratch
COPY --from=builder-stage /pibox-framebuffer /
