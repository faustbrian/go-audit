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
	cd ../.. && scripts/with-gocache.sh ./scripts/check-module.sh . tidy-check
	cd ../.. && scripts/with-gocache.sh ./scripts/check-module.sh postgres tidy-check

test:
	cd ../.. && scripts/with-gocache.sh ./scripts/check-module.sh . test

integration:
	cd ../.. && POSTGRES_VERSION='$(POSTGRES_VERSION)' scripts/with-gocache.sh ./scripts/check-module.sh postgres test

integration-matrix:
	./scripts/run-postgres-matrix.sh

test-race:
	cd ../.. && scripts/with-gocache.sh ./scripts/check-module.sh . race
	cd ../.. && scripts/with-gocache.sh ./scripts/check-module.sh postgres race

coverage:
	./scripts/check-coverage.sh

mutation:
	./scripts/with-gocache.sh ../../scripts/check-mutation.sh .
	./scripts/with-gocache.sh ../../scripts/check-mutation.sh postgres

fuzz:
	./scripts/with-gocache.sh $(GO) test . -run '^$$' -fuzz '^FuzzCanonicalRecord$$' -fuzztime='$(FUZZ_TIME)'
	./scripts/with-gocache.sh $(GO) test . -run '^$$' -fuzz '^FuzzHostileRecordConstruction$$' -fuzztime='$(FUZZ_TIME)'
	./scripts/with-gocache.sh $(GO) test . -run '^$$' -fuzz '^FuzzCursor$$' -fuzztime='$(FUZZ_TIME)'
	cd ../.. && GOLIB_FUZZ_SMOKE_BUDGET='$(FUZZ_TIME)' scripts/with-gocache.sh ./scripts/check-module.sh postgres fuzz

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
	cd ../.. && scripts/with-gocache.sh ./scripts/check-module.sh . vet
	cd ../.. && scripts/with-gocache.sh ./scripts/check-module.sh postgres vet

docs:
	./scripts/check-docs.sh

check: format-check module-check vet test integration-matrix test-race coverage fuzz stress soak mutation benchmark clean-consumer docs
