GOLIB ?= golib

.PHONY: source-docs-check check check-full ci cohesion docs-local inventory repository-check

check: source-docs-check docs-local
	$(GOLIB) check --local --all

docs-local:
	bash scripts/check-docs.sh --local

check-full: source-docs-check
	$(GOLIB) check --all

ci: source-docs-check repository-check cohesion check

cohesion:
	$(GOLIB) cohesion check

inventory:
	$(GOLIB) inventory

repository-check:
	$(GOLIB) repository check

source-docs-check:
	$(GOLIB) source-docs check --json
