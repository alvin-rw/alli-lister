## help: print this help message
.PHONY: help
help:
	@echo 'Usage'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'

## run: run the application
.PHONY: run
run:
	@go run . -debug=false

## debug: run the application in debug mode
.PHONY: debug
debug:
	@go run . -debug=true

## all-regions: run the application to get data from all regions
.PHONY: all-regions
all-regions:
	@go run . -all-regions=true

## build: build the application for multiple platforms
.PHONY: build
build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ./build/alli-lister.linux-amd64
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -o ./build/alli-lister.linux-armv7
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o ./build/alli-lister.linux-arm64
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o ./build/alli-lister.darwin-amd64
	CGO_ENABLED=0 GOOS=windows GOARCH=386 go build -o ./build/alli-lister.windows-386.exe
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o ./build/alli-lister.windows-amd64.exe
