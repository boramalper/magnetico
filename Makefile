.PHONY: test format vet staticcheck magneticod magneticow image image-magneticow image-magneticod

all: test magneticod magneticow

magneticod:
	go install --tags fts5 "-ldflags=-s -w -X main.compiledOn=`date -u +%Y-%m-%dT%H:%M:%SZ`" ./cmd/magneticod

magneticow:
	# TODO: minify files!
	# https://github.com/kevinburke/go-bindata
	go-bindata -o="cmd/magneticow/bindata.go" -prefix="cmd/magneticow/data/" cmd/magneticow/data/...
	go install --tags fts5 "-ldflags=-s -w -X main.compiledOn=`date -u +%Y-%m-%dT%H:%M:%SZ`" ./cmd/magneticow

image-magneticod:
	docker build -t boramalper/magneticod -f Dockerfile.magneticod .

image-magneticow:
	docker build -t boramalper/magneticow -f Dockerfile.magneticow .

image: image-magneticod image-magneticow

vet:
	go vet ./...

staticcheck:
	staticcheck ./...

test:
	go test ./...

format:
	gofmt -w ./cmd/
	gofmt -w ./pkg/

# Formatting Errors
#     Since gofmt returns zero even if there are files to be formatted, we use:
#
#       ! gofmt -d ${GOPATH}/path/ 2>&1 | read
#
#     to return 1 if there are files to be formatted, and 0 if not.
#     https://groups.google.com/forum/#!topic/Golang-Nuts/pdrN4zleUio
#
# How can I use Bash syntax in Makefile targets?
#     Because `read` is a bash command.
#     https://stackoverflow.com/a/589300/4466589
#
check-formatting: SHELL:=/bin/bash   # HERE: this is setting the shell for check-formatting only
check-formatting:
	! gofmt -l ./cmd/ 2>&1 | tee /dev/fd/2 | read
	! gofmt -l ./pkg/ 2>&1 | tee /dev/fd/2 | read
