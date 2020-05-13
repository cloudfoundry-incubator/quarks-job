export PROJECT ?= quarks-job
export QUARKS_UTILS ?= vendor/code.cloudfoundry.org/quarks-utils
export GROUP_VERSIONS ?= quarksjob:v1alpha1

test-unit: vendor
	bash $(QUARKS_UTILS)/bin/test-unit

test-cluster: vendor
	bin/build-image
	bash $(QUARKS_UTILS)/bin/test-integration
	bash $(QUARKS_UTILS)/bin/test-cli-e2e
	bin/build-helm
	bash $(QUARKS_UTILS)/bin/test-helm-e2e

lint: vendor
	bash $(QUARKS_UTILS)/bin/lint

build-image:
	bin/build-image

publish-image:
	bin/build-image
	bin/publish-image

############ GENERATE TARGETS ############

generate: gen-kube

gen-kube:
	bash $(QUARKS_UTILS)/bin/gen-kube

gen-command-docs:
	rm -f docs/commands/*
	go run cmd/docs/gen-command-docs.go

vendor:
	go mod vendor

############ COVERAGE TARGETS ############

coverage: vendor
	bash $(QUARKS_UTILS)/bin/coverage
