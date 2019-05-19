# Start from a Debian image with the latest version of Go installed
# and a workspace (GOPATH) configured at /go.
FROM golang:1.12-alpine AS build

RUN apk add --no-cache build-base curl git

# Copy the local package files to the container's workspace.
ADD ./Makefile        /magnetico/
ADD ./pkg             /magnetico/pkg
ADD ./go.mod          /magnetico/go.mod
ADD ./cmd/magneticod  /magnetico/cmd/magneticod

WORKDIR /magnetico/
RUN     make magneticod

FROM alpine:latest
LABEL maintainer="bora@boramalper.org"
WORKDIR /
COPY --from=build /go/bin/magneticod /magneticod

RUN adduser -D -S magnetico
USER magnetico

ENTRYPOINT ["/magneticod"]
