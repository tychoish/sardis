name := sardis
alt := riker
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


build:$(buildDir)/$(name) $(buildDir)/$(alt)

$(name):$(buildDir)/$(name)
	ln -sf $(buildDir)/$(name) || true

$(buildDir)/$(name):$(srcFiles) go.mod go.sum
	@mkdir -p $(buildDir)
	go build -ldflags "-X github.com/tychoish/sardis.BuildRevision=`git rev-parse HEAD`" -o $@ cmd/$(name)/$(name).go

$(alt):$(buildDir)/$(alt)
	ln -fs $(buildDir)/$(alt)

$(buildDir)/$(alt):$(srcFiles) go.mod go.sum
	@mkdir -p $(buildDir)
	go build -ldflags "-X github.com/tychoish/sardis.BuildRevision=`git rev-parse HEAD`" -o $@ cmd/$(alt)/$(alt).go


benchmark:
	go test -v -benchmem $(benchArgs) -timeout=20m ./...
