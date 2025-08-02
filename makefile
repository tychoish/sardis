name := sardis
buildDir := build
srcFiles := $(shell find . -name "*.go" -not -path "./$(buildDir)/*" -not -name "*_test.go" -not -path "*\#*")
testFiles := $(shell find . -name "*.go" -not -path "./$(buildDir)/*" -not -path "*\#*")

_testPackages := ./ ./units ./operations

MAKEFLAGS := --jobs 2

ifeq (,$(SILENT))
testArgs := -v
endif

ifneq (,$(RUN_BENCH))
benchArgs += -bench="$(RUN_BENCH)"
benchArgs += -run='$(RUN_BENCH)'
else
benchArgs += -bench=.
benchArgs += -run='Benchmark.*'
endif

GIT_REV := $(shell git rev-parse HEAD)
BUILD_TIME := $(shell date +'%Y-%m-%d.%H:%M:%S')

ifneq (,$(RELEASE))
LDFLAGS := -X 'github.com/tychoish/sardis/global.buildRevision=$(GIT_REV)' -X 'github.com/tychoish/sardis/global.buildTimeString=$(BUILD_TIME)'
BUILD_FLAGS := -ldflags="$(LDFLAGS)" -o
else
BUILD_FLAGS := -o
endif


build:$(buildDir)/$(name)

$(name):$(buildDir)/$(name)
	ln -sf $(buildDir)/$(name) || true

$(buildDir)/$(name):$(srcFiles) go.mod go.sum
	@mkdir -p $(buildDir)
	go build $(BUILD_FLAGS) $@ cmd/$(name)/$(name).go

benchmark:
	go test -v -benchmem $(benchArgs) -timeout=20m ./...
