.PHONY: test format magneticod magneticow ensure test-magneticod test-magneticow test-persistence image image-magneticow image-magneticod

all: ensure test magneticod magneticow

magneticod:
	go install --tags fts5 "-ldflags=-s -w" github.com/boramalper/magnetico/cmd/magneticod

magneticow:
	# TODO: minify files!
	go-bindata -o="cmd/magneticow/bindata.go" -prefix="cmd/magneticow/data/" -pkg="main" cmd/magneticow/data/...
	go install --tags fts5 "-ldflags=-s -w" github.com/boramalper/magnetico/cmd/magneticow

image-magneticod:
	docker build -t magneticod -f Dockerfile.magneticod .

image-magneticow:
	docker build -t magneticow -f Dockerfile.magneticow .

image: image-magneticod image-magneticow

# Download dependencies
ensure:
	dep ensure -v
	go get -u github.com/kevinburke/go-bindata/...

test-magneticod:
	go test github.com/boramalper/magnetico/cmd/magneticod/...

test-magneticow:
	go-bindata -o="cmd/magneticow/bindata.go" -prefix="cmd/magneticow/data/" -pkg="main" cmd/magneticow/data/...
	go test github.com/boramalper/magnetico/cmd/magneticow/...

test-persistence:
	go test github.com/boramalper/magnetico/pkg/persistence/...

test: test-persistence test-magneticod test-magneticow

format:
	gofmt -w cmd/magneticod
	gofmt -w cmd/magneticow
	gofmt -w pkg/persistence

