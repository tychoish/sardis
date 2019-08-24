name := sardis
buildDir := build
srcFiles := $(shell find . -name "*.go" -not -path "./$(buildDir)/*" -not -name "*_test.go" -not -path "*\#*")
testFiles := $(shell find . -name "*.go" -not -path "./$(buildDir)/*" -not -path "*\#*")

_testPackages := ./ ./units ./operations

ifeq (,$(SILENT))
testArgs := -v
endif

ifneq (,$(RUN_TEST))
testArgs += -run='$(RUN_TEST)'
endif

ifneq (,$(RUN_COUNT))
testArgs += -count='$(RUN_COUNT)'
endif
ifneq (,$(SKIP_LONG))
testArgs += -short
endif
ifneq (,$(DISABLE_COVERAGE))
testArgs += -cover
endif
ifneq (,$(RACE_DETECTOR))
testArgs += -race
endif

ifneq (,$(RUN_BENCH))
benchArgs += -bench="$(RUN_BENCH)"
benchArgs += -run='$(RUN_BENCH)'
else
benchArgs += -bench=.
benchArgs += -run='Benchmark.*'
endif


build:$(buildDir)/$(name)
$(name):$(buildDir)/$(name)
	ln -s $(buildDir)/$(name)
$(buildDir)/$(name):$(srcFiles)
	@mkdir -p $(buildDir)
	go build -ldflags "-X github.com/tychoish/sardis.BuildRevision=`git rev-parse HEAD`" -o $@ cmd/$(name)/main.go

test:
	@mkdir -p $(buildDir)
	go test $(testArgs) $(_testPackages) | tee $(buildDir)/test.ftdc.out
	@grep -s -q -e "^PASS" $(buildDir)/test.ftdc.out
coverage:$(buildDir)/cover.out
	@go tool cover -func=$< | sed -E 's%github.com/.*/$(name)/%%' | column -t
coverage-html:$(buildDir)/cover.html 

benchmark:
	go test -v -benchmem $(benchArgs) -timeout=20m ./...


$(buildDir)/cover.out:$(buildDir) $(testFiles) .FORCE
	go test $(testArgs) -covermode=count -coverprofile $@ -cover ./
$(buildDir)/cover.html:$(buildDir)/cover.out
	go tool cover -html=$< -o $@


test-%:
	@mkdir -p $(buildDir)
	go test $(testArgs) ./$* | tee $(buildDir)/test.*.out
	@grep -s -q -e "^PASS" $(buildDir)/test.*.out
coverage-%:$(buildDir)/cover.%.out
	@go tool cover -func=$< | sed -E 's%github.com/.*/sardis/%%' | column -t
html-coverage-%:coverage-%
$(buildDir)/cover.%.out:$(buildDir) $(testFiles) .FORCE
	go test $(testArgs) -covermode=count -coverprofile $@ -cover ./$*
$(buildDir)/cover.%.html:$(buildDir)/cover.%.out
	go tool cover -html=$< -o $@

.DEFAULT:$(buildDir)/$(name)
.PHONY:build test
.FORCE:

vendor-clean:
	find vendor/ -name "*.gif" -o -name "*.jpg" -o -name "*.gz" -o -name "*.png" -o -name "*.ico" | xargs rm -rf
	rm -rf vendor/github.com/mongodb/grip/vendor/github.com/stretchr/
	rm -rf vendor/github.com/mongodb/grip/vendor/github.com/pkg/errors/
	rm -rf vendor/gopkg.in/mgo.v2/harness/
	rm -rf vendor/gopkg.in/mgo.v2/testdb/
	rm -rf vendor/gopkg.in/mgo.v2/testserver/
	rm -rf vendor/gopkg.in/mgo.v2/internal/json/testdata
	rm -rf vendor/gopkg.in/mgo.v2/.git/
	rm -rf vendor/gopkg.in/mgo.v2/txn/
	rm -rf vendor/github.com/mongodb/amboy/vendor/github.com/pkg/
	rm -rf vendor/github.com/mongodb/amboy/vendor/github.com/mongodb/grip/
	rm -rf vendor/github.com/mongodb/amboy/vendor/github.com/stretchr/
	rm -rf vendor/github.com/mongodb/grip/vendor/github.com/stretchr/
	rm -rf vendor/github.com/mongodb/jasper/vendor/github.com/mongodb/grip/
	rm -rf vendor/github.com/mongodb/jasper/vendor/github.com/mongodb/amboy/
	rm -rf vendor/github.com/mongodb/jasper/vendor/github.com/pkg/
	rm -rf vendor/github.com/mongodb/jasper/vendor/github.com/stretchr/
	rm -rf vendor/github.com/mongodb/jasper/vendor/github.com/urfave/cli
	rm -rf vendor/github.com/mongodb/amboy/vendor/github.com/urfave/cli

.DEFAULT:build
.PHONY:build test
.FORCE:
