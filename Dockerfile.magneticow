# Start from a Debian image with the latest version of Go installed
# and a workspace (GOPATH) configured at /go.
FROM golang:1.12-alpine AS build

RUN export PATH=$PATH:/go/bin

RUN apk add --no-cache build-base curl git

# Copy the local package files to the container's workspace.
ADD ./Makefile        /magnetico/
ADD ./pkg             /magnetico/pkg
ADD ./go.mod          /magnetico/go.mod
ADD ./cmd/magneticow  /magnetico/cmd/magneticow

WORKDIR /magnetico

RUN go get -u github.com/kevinburke/go-bindata/...

RUN echo $PATH
RUN ls /go/bin

RUN     make magneticow

FROM alpine:latest
LABEL maintainer="bora@boramalper.org"
WORKDIR /
COPY --from=build /go/bin/magneticow /magneticow

RUN adduser -D -S magnetico
USER magnetico

ENTRYPOINT ["/magneticow"]

# Document that the service listens on port 8080.
EXPOSE 8080
