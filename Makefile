# Shortcuts for the common tasks. `make verify` is the important one — it runs
# exactly what CI runs, so a green `make verify` should mean a green build.
#
# Note: the indented lines below must use TAB characters, not spaces. That's a
# Make quirk, not a style choice.
#
# This repo is two independent Go modules, not one: the CLI under ./cli and the
# Lambda handler under ./lambda, each with its own go.mod. There is no module at
# the repo root, so every Go command has to run inside one of them — hence the
# loops below and the `go -C` calls.

MODULES := cli lambda

.PHONY: help verify fmt fmt-check build vet test lint clean install snapshot

# `make` with no argument prints this.
help:
	@echo "make verify    - format check, build, vet, test (what CI runs)"
	@echo "make fmt       - format the code in place"
	@echo "make build     - compile the CLI binary to ./doi-dropbox"
	@echo "make test      - run tests with the race detector"
	@echo "make lint      - run golangci-lint (must be installed)"
	@echo "make snapshot  - build all release platforms locally, no publish"
	@echo "make install   - install the CLI to your GOPATH/bin"
	@echo "make clean     - remove build artifacts"

# The one to run before pushing.
verify: fmt-check build vet test

# Fails if anything is unformatted, and names the offenders. This mirrors the
# CI step rather than silently reformatting, so local and CI behavior match.
# gofmt walks the tree on its own, so this one does not need the module loop.
fmt-check:
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "These files need gofmt:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi
	@echo "formatting: ok"

fmt:
	gofmt -w .

# Only the CLI is a distributable binary; the Lambda handler is packaged by the
# deploy tooling. `go -C` builds from ./cli while leaving the output at the root.
build:
	go -C cli build -o ../doi-dropbox .

vet:
	@for m in $(MODULES); do \
		echo "==> vet $$m"; \
		(cd $$m && go vet ./...) || exit 1; \
	done

test:
	@for m in $(MODULES); do \
		echo "==> test $$m"; \
		(cd $$m && go test ./... -race -cover) || exit 1; \
	done

lint:
	@for m in $(MODULES); do \
		echo "==> lint $$m"; \
		(cd $$m && golangci-lint run) || exit 1; \
	done

# Build every release platform locally without publishing anything. Useful
# before tagging — see .claude/skills/release-cli/SKILL.md. GoReleaser is
# configured with `dir: cli`, so this one runs from the root.
snapshot:
	goreleaser build --snapshot --clean

install:
	go -C cli install .

# Also removes the per-module binaries: `go build ./...` inside a main package
# leaves one named after the directory, which is easy to commit by accident.
clean:
	rm -f doi-dropbox
	rm -f $(addsuffix /coverage.out,$(MODULES))
	rm -f $(join $(addsuffix /,$(MODULES)),$(MODULES))
	rm -rf dist/
