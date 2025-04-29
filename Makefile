COVERAGE_DIR 		= coverage
COVERAGE_FNAME 		= cover.html

COVERAGE_SRC 		= $(COVERAGE_DIR)/coverage.out
COVERAGE_DST 		= $(COVERAGE_DIR)/$(COVERAGE_FNAME)

# compilation flags
GO_VARS 			= 	CGO_ENABLED=0
GO_FLAGS 			= 	-trimpath
GC_FLAGS			=   -gcflags="-m=2 -l"

APP 				= 	fsyncd

.PHONY: 			all build clear

build:
	$(GO_VARS) go build $(GO_FLAGS) -v -o $(APP) .

run_test:
	go test -v .

run_multiple_test:
	go test -count=100 .

run_vet:
	go vet .

build_test_coverage:
	go test ./... -coverprofile=$(COVERAGE_SRC) -coverpkg ./...
	go tool cover -html=$(COVERAGE_SRC) -o $(COVERAGE_DST)

clear:
	rm -rf $(APP)