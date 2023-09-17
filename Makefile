override APP_NAME=squirrelup
override GO_VERSION=1.21
override GOLANGCI_LINT_VERSION=v1.54.2
override SECUREGO_GOSEC_VERSION=2.17.0
override HADOLINT_VERSION=v2.10.0

GOOS?=$(shell go env GOOS || echo linux)
GOARCH?=$(shell go env GOARCH || echo amd64)
CGO_ENABLED?=0

ifeq (, $(shell which docker))
$(error "Binary docker not found in $(PATH)")
endif

.PHONY: all
all: cleanup vendor lint test build

.PHONY: cleanup
cleanup:
	@docker run --rm \
		-v ${PWD}:/project \
		-w /project \
		debian:stable-slim \
		rm bin/${APP_NAME} coverage.out vendor

.PHONY: tidy
tidy:
	@docker run --rm \
		-v ${PWD}:/project \
		-w /project \
		golang:${GO_VERSION} \
			go mod tidy

.PHONY: vendor
vendor:
	@docker run --rm \
		-v ${PWD}:/project \
		-w /project \
		golang:${GO_VERSION} \
			go mod vendor

.PHONY: lint-golangci-lint
lint-golangci-lint:
	@docker run --rm \
		-v ${PWD}:/project \
		-w /project \
		golangci/golangci-lint:${GOLANGCI_LINT_VERSION} \
			golangci-lint run -v

.PHONY: lint-gosec
lint-gosec:
	@docker run --rm \
		-v ${PWD}:/project \
		-w /project \
		securego/gosec:${SECUREGO_GOSEC_VERSION} \
			-exclude-dir=.go-mod \
			-exclude-dir=.go-build \
			/project/... \

.PHONY: lint
lint:
	@make lint-golangci-lint
	@make lint-gosec

.PHONY: fmt
fmt:
	@docker run --rm \
		-v ${PWD}:/project \
		-w /project \
		golang:${GO_VERSION} \
			go fmt /project/cmd/... /project/pkg/...

.PHONY: test
test:
	@mkdir -p ${PWD}/.go-build ${PWD}/.go-mod
	@docker run --rm \
		-v ${PWD}:/project \
		-w /project \
		golang:${GO_VERSION} \
			go test \
				-race \
				-mod vendor \
				-covermode=atomic \
				-coverprofile=/project/coverage.out \
					/project/...
	@docker run --rm \
		-v ${PWD}:/project \
		-w /project \
		golang:${GO_VERSION} \
			go tool cover \
				-o coverage.html \
				-html=coverage.out

.PHONY: build
build:
	@mkdir -p ${PWD}/.go-build ${PWD}/.go-mod
	@docker run --rm \
		-v ${PWD}:/project \
		-v ${PWD}/.go-build:/root/.cache/go-build \
		-v ${PWD}/.go-mod:/go/pkg/mod \
		-w /project \
		-e GOOS=${GOOS} \
		-e GOARCH=${GOARCH} \
		-e CGO_ENABLED=${CGO_ENABLED} \
		-e GO111MODULE=on \
		golang:${GO_VERSION} \
			go build \
				-mod vendor \
				-o /project/bin/${APP_NAME} \
				-v /project/cmd/${APP_NAME}

.PHONY: get
get:
	@docker run --rm \
		-v ${PWD}:/project \
		-v ${PWD}/.go-build:/root/.cache/go-build \
		-v ${PWD}/.go-mod:/go/pkg/mod \
		-w /project \
		-e GOOS=${GOOS} \
		-e GOARCH=${GOARCH} \
		-e CGO_ENABLED=${CGO_ENABLED} \
		-e GO111MODULE=on \
		golang:${GO_VERSION} \
			go get ${GOPKG}
