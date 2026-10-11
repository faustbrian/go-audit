GOLIB ?= golib

.PHONY: check check-full ci cohesion docs-local inventory repository-check

check: docs-local
	$(GOLIB) check --local --all

docs-local:
	bash scripts/check-docs.sh --local

check-full:
	$(GOLIB) check --all

ci: repository-check cohesion check

cohesion:
	$(GOLIB) cohesion check

inventory:
	$(GOLIB) inventory

repository-check:
	$(GOLIB) repository check
