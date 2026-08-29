.PHONY: clean-consumer docs

clean-consumer:
	./scripts/check-clean-consumer.sh

docs:
	./scripts/check-docs.sh
