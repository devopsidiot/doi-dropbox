# Shortcuts for the common tasks. `make verify` is the important one — it runs
# exactly what CI runs, so a green `make verify` should mean a green build.
#
# Note: the indented lines below must use TAB characters, not spaces. That's a
# Make quirk, not a style choice.

.PHONY: help verify fmt build vet test lint clean install snapshot

# `make` with no argument prints this.
help:
	@echo "make verify    - format check, build, vet, test (what CI runs)"
	@echo "make fmt       - format the code in place"
	@echo "make build     - compile the binary to ./doi-dropbox"
	@echo "make test      - run tests with the race detector"
	@echo "make lint      - run golangci-lint (must be installed)"
	@echo "make snapshot  - build all release platforms locally, no publish"
	@echo "make install   - install the binary to your GOPATH/bin"
	@echo "make clean     - remove build artifacts"

# The one to run before pushing.
verify: fmt-check build vet test

# Fails if anything is unformatted, and names the offenders. This mirrors the
# CI step rather than silently reformatting, so local and CI behavior match.
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

build:
	go build -o doi-dropbox .

vet:
	go vet ./...

test:
	go test ./... -race -cover

lint:
	golangci-lint run

# Build every release platform locally without publishing anything. Useful
# before tagging — see .claude/skills/release-cli/SKILL.md.
snapshot:
	goreleaser build --snapshot --clean

install:
	go install .

clean:
	rm -f doi-dropbox coverage.out
	rm -rf dist/
