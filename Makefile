APP=inventory-back
PKG=./...
BIN=bin/api
VERSION ?= dev
IMAGE ?= ghcr.io/jonathancaamano/$(APP):$(VERSION)

GOLANGCI_LINT_VERSION ?= v1.64.8
MIGRATE ?= migrate
DB_URL ?= $(DATABASE_URL)

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: tidy-check
tidy-check:
	go mod tidy
	git diff --exit-code go.mod go.sum

.PHONY: fmt
fmt:
	gofmt -w .

.PHONY: fmt-check
fmt-check:
	@test -z "$$(gofmt -l .)" || (echo "gofmt needed"; gofmt -l .; exit 1)

.PHONY: vet
vet:
	go vet $(PKG)

.PHONY: lint
lint:
	@command -v golangci-lint >/dev/null 2>&1 || (echo "golangci-lint not found"; exit 1)
	golangci-lint run

.PHONY: lint-install
lint-install:
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin $(GOLANGCI_LINT_VERSION)

.PHONY: test
test:
	go test $(PKG)

.PHONY: build
build:
	go build -o $(BIN) ./cmd/server

.PHONY: run
run:
	go run ./cmd/server

.PHONY: docker-build
docker-build:
	docker build -t $(IMAGE) .

.PHONY: docker-push
docker-push:
	docker push $(IMAGE)

.PHONY: migrate-up
migrate-up:
	$(MIGRATE) -path ./migrations -database "$(DB_URL)" up

.PHONY: migrate-down
migrate-down:
	$(MIGRATE) -path ./migrations -database "$(DB_URL)" down 1

.PHONY: check
check: fmt vet lint test