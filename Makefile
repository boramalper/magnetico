all: magneticod magneticow

magneticod:
	go install magnetico/magneticod

magneticow:
	# TODO: minify files!
	go-bindata -o="magneticow/bindata.go" -prefix="magneticow/data/" magneticow/data/...
	go install magnetico/magneticow
