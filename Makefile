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