GO ?= go
POSTGRES_VERSION ?= 18
BENCH_TIME ?= 100ms
FUZZ_TIME ?= 2s

.PHONY: benchmark check clean-consumer coverage docs format format-check fuzz integration integration-matrix module-check mutation soak stress test test-race vet

format:
	gofmt -w $$(find . -type f -name '*.go')

format-check:
	test -z "$$(gofmt -l $$(find . -type f -name '*.go'))"

module-check:
	./scripts/with-gocache.sh ./.golib/scripts/run-modules.sh tidy-check --all

test:
	./scripts/with-gocache.sh ./.golib/scripts/run-modules.sh test --all

integration:
	./scripts/run-postgres-matrix.sh

integration-matrix:
	./scripts/run-postgres-matrix.sh

test-race:
	./scripts/with-gocache.sh ./.golib/scripts/run-modules.sh race --all

coverage:
	./scripts/check-coverage.sh

mutation:
	./scripts/with-gocache.sh ./.golib/scripts/check-mutation.sh .
	./scripts/with-gocache.sh ./.golib/scripts/check-mutation.sh postgres

fuzz:
	./scripts/with-gocache.sh $(GO) test . -run '^$$' -fuzz '^FuzzCanonicalRecord$$' -fuzztime='$(FUZZ_TIME)'
	./scripts/with-gocache.sh $(GO) test . -run '^$$' -fuzz '^FuzzHostileRecordConstruction$$' -fuzztime='$(FUZZ_TIME)'
	./scripts/with-gocache.sh $(GO) test . -run '^$$' -fuzz '^FuzzCursor$$' -fuzztime='$(FUZZ_TIME)'

stress:
	./scripts/with-gocache.sh $(GO) test ./memory -run 'Stress' -count=100
	cd postgres && ../scripts/with-gocache.sh $(GO) test -tags=integration . -run 'TestPostgreSQLAppendQueryIdempotencyAndWriterPrivileges' -count=1

soak:
	./scripts/with-gocache.sh $(GO) test ./memory -run 'Soak' -count=25

benchmark:
	./scripts/with-gocache.sh $(GO) test ./... -run '^$$' -bench . -benchmem -benchtime='$(BENCH_TIME)'
	cd postgres && POSTGRES_VERSION='$(POSTGRES_VERSION)' ../scripts/with-gocache.sh $(GO) test -tags=integration . -run '^$$' -bench . -benchmem -benchtime='$(BENCH_TIME)'

clean-consumer:
	./scripts/with-gocache.sh ./scripts/check-clean-consumer.sh

vet:
	./scripts/with-gocache.sh ./.golib/scripts/run-modules.sh vet --all

docs:
	./scripts/check-docs.sh

check: format-check module-check vet test integration-matrix test-race coverage fuzz stress soak mutation benchmark clean-consumer docs
