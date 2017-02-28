# start project configuration
name := sardis
buildDir := build
packages := $(name) rest units operations
orgPath := github.com/tychoish
projectPath := $(orgPath)/$(name)
# end project configuration


# dependencies
deps := github.com/mongodb/grip
deps += github.com/mongodb/amboy
deps += github.com/tychoish/gimlet
deps += github.com/pkg/errors
# end dependencies


# start linting configuration
#   package, testing, and linter dependencies specified
#   separately. This is a temporary solution: eventually we should
#   vendorize all of these dependencies.
lintDeps := github.com/alecthomas/gometalinter
#   include test files and give linters 40s to run to avoid timeouts
lintArgs := --tests --deadline=14m --vendor --concurrency=3
#   gotype produces false positives because it reads .a files which
#   are rarely up to date.
lintArgs += --disable="gotype" --disable="gas"
lintArgs += --skip="build" --skip="buildscripts"
#   enable and configure additional linters
lintArgs += --enable="goimports"
lintArgs += --linter='misspell:misspell ./*.go:PATH:LINE:COL:MESSAGE' --enable=misspell
lintArgs += --line-length=100 --dupl-threshold=150 --cyclo-over=15
#   the gotype linter has an imperfect compilation simulator and
#   produces the following false postive errors:
lintArgs += --exclude="error: could not import github.com/mongodb/greenbay"
#   some test cases are structurally similar, and lead to dupl linter
#   warnings, but are important to maintain separately, and would be
#   difficult to test without a much more complex reflection/code
#   generation approach, so we ignore dupl errors in tests.
lintArgs += --exclude="warning: duplicate of .*_test.go"
#   go lint warns on an error in docstring format, erroneously because
#   it doesn't consider the entire package.
lintArgs += --exclude="warning: package comment should be of the form \"Package .* ...\""
#   known issues that the linter picks up that are not relevant in our cases
lintArgs += --exclude="file is not goimported" # top-level mains aren't imported
lintArgs += --exclude="error return value not checked .defer.*"
lintArgs += --exclude="\w+Key is unused.*"
lintArgs += --exclude="unused global variable \w+Key"
# end linting configuration


# start dependency installation tools
#   implementation details for being able to lazily install dependencies
gopath := $(shell go env GOPATH)
lintDeps := $(addprefix $(gopath)/src/,$(lintDeps))
deps := $(addprefix $(gopath)/src/,$(deps))
srcFiles := makefile $(shell find . -name "*.go" -not -path "./$(buildDir)/*" -not -name "*_test.go" -not -path "./buildscripts/*" )
testSrcFiles := makefile $(shell find . -name "*.go" -not -path "./$(buildDir)/*")
testOutput := $(foreach target,$(packages),$(buildDir)/output.$(target).test)
raceOutput := $(foreach target,$(packages),$(buildDir)/output.$(target).race)
coverageOutput := $(foreach target,$(packages),$(buildDir)/output.$(target).coverage)
coverageHtmlOutput := $(foreach target,$(packages),$(buildDir)/output.$(target).coverage.html)
$(gopath)/src/%:
	@-[ ! -d $(gopath) ] && mkdir -p $(gopath) || true
	go get $(subst $(gopath)/src/,,$@)
makefile:$(deps)
# end dependency installation tools


# implementation details for building the binary and creating a
# convienent link in the working directory
define crossCompile
	@./$(buildDir)/build-cross-compile -buildName=$* -ldflags="-X=github.com/tychoish/sink.BuildRevision=`git rev-parse HEAD`" -goBinary="`which go`" -output=$@
endef
$(name):$(buildDir)/$(name)
	@[ -e $@ ] || ln -s $<
$(buildDir)/$(name):$(srcFiles)
	go build -ldflags "-X github.com/tychoish/sink.BuildRevision=`git rev-parse HEAD`" -o $@ main/$(name).go
$(buildDir)/$(name).race:$(srcFiles)
	go build -race -ldflags "-X github.com/tychoish/sink.BuildRevision=`git rev-parse HEAD`" -o $@ main/$(name).go
# end dependency installation tools


# distribution targets and implementation
distContents := $(agentBuildDir) $(clientBuildDir) $(distArtifacts)
distTestContents := $(foreach pkg,$(packages),$(buildDir)/test.$(pkg) $(buildDir)/race.$(pkg))
$(buildDir)/build-cross-compile:buildscripts/build-cross-compile.go
	@mkdir -p $(buildDir)
	go build -o $@ $<
$(buildDir)/make-tarball:buildscripts/make-tarball.go $(buildDir)/render-gopath
	go build -o $@ $<
dist:$(buildDir)/dist.tar.gz
dist-test:$(buildDir)/dist-test.tar.gz
dist-race: $(buildDir)/dist-race.tar.gz
dist-source:$(buildDir)/dist-source.tar.gz
$(buildDir)/dist.tar.gz:$(buildDir)/make-tarball $(binaries)
	./$< --name $@ --prefix $(name) $(foreach item,$(binaries) $(distContents),--item $(item))
$(buildDir)/dist-race.tar.gz:$(buildDir)/make-tarball makefile $(raceBinaries)
	./$< -name $@ --prefix $(name)-race $(foreach item,$(raceBinaries) $(distContents),--item $(item))
$(buildDir)/dist-test.tar.gz:$(buildDir)/make-tarball makefile $(binaries) $(raceBinaries)
	./$< -name $@ --prefix $(name)-tests $(foreach item,$(distContents) $(distTestContents),--item $(item)) $(foreach item,,--item $(item))
$(buildDir)/dist-source.tar.gz:$(buildDir)/make-tarball $(srcFiles) $(testSrcFiles) makefile
	./$< --name $@ --prefix $(name) $(subst $(name),,$(foreach pkg,$(packages),--item ./$(subst -,/,$(pkg)))) --item ./scripts --item makefile --exclude "$(name)" --exclude "^.git/" --exclude "$(buildDir)/"
# end main build


# userfacing targets for basic build and development operations
lint:$(lintDeps)
	@-$(gopath)/bin/gometalinter --install >/dev/null
	$(gopath)/bin/gometalinter $(lintArgs) ./...
build:$(buildDir)/$(name)
build-race:$(buildDir)/$(name).race
test:$(foreach target,$(packages),test-$(target))
race:$(foreach target,$(packages),race-$(target))
coverage:$(coverageOutput)
coverage-html:$(coverageHtmlOutput)
list-tests:
	@echo -e "test targets:" $(foreach target,$(packages),\\n\\ttest-$(target))
list-race:
	@echo -e "test (race detector) targets:" $(foreach target,$(packages),\\n\\trace-$(target))
phony += lint lint-deps build build-race race test coverage coverage-html list-race list-tests
.PRECIOUS: $(testOutput) $(raceOutput) $(coverageOutput) $(coverageHtmlOutput)
.PRECIOUS: $(foreach target,$(packages),$(buildDir)/test.$(target))
.PRECIOUS: $(foreach target,$(packages),$(buildDir)/race.$(target))
# end front-ends


# convenience targets for runing tests and coverage tasks on a
# specific package.
race-%:$(buildDir)/output.%.race
	@grep -s -q -e "^PASS" $< && ! grep -s -q "^WARNING: DATA RACE" $<
test-%:$(buildDir)/output.%.test
	@grep -s -q -e "^PASS" $<
coverage-%:$(buildDir)/output.%.coverage
	@grep -s -q -e "^PASS" $<
html-coverage-%:$(buildDir)/output.%.coverage.html $(buildDir)/output.%.coverage.html
	@grep -s -q -e "^PASS" $<
# end convienence targets


# start test and coverage artifacts
#    This varable includes everything that the tests actually need to
#    run. (The "build" target is intentional and makes these targetsb
#    rerun as expected.)
testRunDeps := $(name)
testTimeout := --test.timeout=20m
testArgs := -test.v $(testTimeout)
#  targets to compile
$(buildDir)/test.%:$(testSrcFiles)
	go test $(if $(DISABLE_COVERAGE),,-covermode=count) -c -o $@ ./$(subst -,/,$*)
$(buildDir)/race.%:$(testSrcFiles)
	go test -race -c -o $@ ./$(subst -,/,$*)
#  targets to run any tests in the top-level package
$(buildDir)/test.$(name):$(testSrcFiles)
	go test $(if $(DISABLE_COVERAGE),,-covermode=count) -c -o $@ ./
$(buildDir)/race.$(name):$(testSrcFiles)
	go test -race -c -o $@ ./
#  targets to run the tests and report the output
$(buildDir)/output.%.test:$(buildDir)/test.% .FORCE
	./$< $(testArgs) 2>&1 | tee $@
$(buildDir)/output.%.race:$(buildDir)/race.% .FORCE
	./$< $(testArgs) 2>&1 | tee $@
#  targets to process and generate coverage reports
$(buildDir)/output.%.coverage:$(buildDir)/test.% .FORCE
	./$< $(testTimeout) -test.coverprofile=$@ || true
	@-[ -f $@ ] && go tool cover -func=$@ | sed 's%$(projectPath)/%%' | column -t
$(buildDir)/output.%.coverage.html:$(buildDir)/output.%.coverage
	go tool cover -html=$< -o $@
# end test and coverage artifacts

# clean and other utility targets
clean:
	rm -rf $(lintDeps) $(buildDir)/test.* $(buildDir)/coverage.* $(buildDir)/race.* $(name) $(buildDir)/$(name)
phony += clean
# end dependency targets

# configure phony targets
.FORCE:
.PHONY:$(phony) .FORCE
