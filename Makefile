test-unit:
	bin/test-unit

test-cluster:
	bin/build-image
	bin/test-integration
	bin/test-cli-e2e
	bin/build-helm
	bin/test-helm-e2e

lint:
	bin/lint

build-image:
	bin/build-image

publish-image:
	bin/build-image
	bin/publish-image

############ GENERATE TARGETS ############

generate: gen-kube

gen-kube:
	bin/gen-kube

gen-command-docs:
	rm -f docs/commands/*
	go run cmd/docs/gen-command-docs.go

############ COVERAGE TARGETS ############

coverage:
	bin/coverage
