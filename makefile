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


compile:
	go build $(_testPackages)
test:
	@mkdir -p $(buildDir)
	go test $(testArgs) $(_testPackages) | tee $(buildDir)/test.ftdc.out
	@grep -s -q -e "^PASS" $(buildDir)/test.ftdc.out
coverage:$(buildDir)/cover.out
	@go tool cover -func=$< | sed -E 's%github.com/.*/ftdc/%%' | column -t
coverage-html:$(buildDir)/cover.html $(buildDir)/cover.bsonx.html

benchmark:
	go test -v -benchmem $(benchArgs) -timeout=20m ./...

$(buildDir):$(srcFiles) compile
	@mkdir -p $@
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

.FORCE:
