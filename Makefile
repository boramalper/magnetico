all: magneticod magneticow

magneticod:
	go install magnetico/magneticod

magneticow:
	# TODO: minify files!
	go-bindata -o="magneticow/bindata.go" -prefix="magneticow/data/" magneticow/data/...
	go install magnetico/magneticow

test:
	go test github.com/boramalper/magnetico/cmd/magneticod/...
	@echo
	go test github.com/boramalper/magnetico/cmd/magneticow/...
	@echo
	go test github.com/boramalper/magnetico/pkg/persistence/...

format:
	gofmt -w cmd/magneticod
	gofmt -w cmd/magneticow
	gofmt -w pkg/persistence

