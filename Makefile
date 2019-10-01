test-unit:
	bin/test-unit

test-cluster:
	bin/test-integration
	bin/test-cli-e2e
	bin/test-helm-e2e

lint:
	bin/lint
