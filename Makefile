GOHOSTOS := $(shell go env GOHOSTOS)
GOPATH := $(shell go env GOPATH)
VERSION=$(shell git describe --tags --always)

ifeq ($(GOHOSTOS), windows)
	Git_Bash=$(subst \,/,$(subst cmd\,bin\bash.exe,$(dir $(shell where git))))
	API_PROTO_FILES=$(shell $(Git_Bash) -c "find api -name *.proto")
else
	API_PROTO_FILES=$(shell find api -name *.proto)
endif

.PHONY: init
# init env
init:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install github.com/go-kratos/kratos/cmd/kratos/v2@latest
	go install github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v2@latest
	go install github.com/google/gnostic/cmd/protoc-gen-openapi@latest
	go install github.com/google/wire/cmd/wire@latest
	go install github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	go install github.com/envoyproxy/protoc-gen-validate@latest

.PHONY: config
# generate internal proto
config:
	protoc --proto_path=./internal \
	       --proto_path=./third_party \
	       --go_out=paths=source_relative:./internal \
	       $(shell find internal/conf -name *.proto)

.PHONY: api
# generate api proto
api:
	protoc --proto_path=./api \
	       --proto_path=./third_party \
	       --go_out=paths=source_relative:./api \
	       --validate_out=lang=go,paths=source_relative:./api \
	       --go-http_out=paths=source_relative:./api \
	       --go-grpc_out=paths=source_relative:./api \
	       --openapi_out=fq_schema_naming=true,default_response=false:. \
	       $(API_PROTO_FILES)

.PHONY: build
# build
build:
	mkdir -p bin/ && go build -ldflags "-X main.Version=$(VERSION)" -o ./bin/ ./cmd/...

.PHONY: generate
# generate
generate:
	go generate ./...
	go mod tidy

.PHONY: all
# generate all
all:
	make api
	make config
	make generate

.PHONY: run
# run dev server
run:
	go run ./cmd/app -conf ./configs/config.yaml

.PHONY: migrate-up
# run database migration up
migrate-up:
	@DB_URL=$$(grep 'source:' configs/config.yaml | sed 's/.*source: *//' | tr -d '"' | head -1); \
	if [ -z "$$DB_URL" ]; then echo "ERROR: could not find database source in configs/config.yaml"; exit 1; fi; \
	echo "Running migrations up with: $$DB_URL"; \
	migrate -path ./migrations -database "$$DB_URL" up

.PHONY: migrate-down
# run database migration down
migrate-down:
	@DB_URL=$$(grep 'source:' configs/config.yaml | sed 's/.*source: *//' | tr -d '"' | head -1); \
	if [ -z "$$DB_URL" ]; then echo "ERROR: could not find database source in configs/config.yaml"; exit 1; fi; \
	echo "Running migrations down with: $$DB_URL"; \
	migrate -path ./migrations -database "$$DB_URL" down 1

.PHONY: migrate-create
# create new migration file: make migrate-create name=xxx
migrate-create:
	migrate create -ext sql -dir ./migrations -seq $(name)

.PHONY: test
# run tests
test:
	go test ./...

# show help
help:
	@echo ''
	@echo 'Usage:'
	@echo ' make [target]'
	@echo ''
	@echo 'Targets:'
	@awk '/^[a-zA-Z\-\_0-9]+:/ { \
	helpMessage = match(lastLine, /^# (.*)/); \
		if (helpMessage) { \
			helpCommand = substr($$1, 0, index($$1, ":")); \
			helpMessage = substr(lastLine, RSTART + 2, RLENGTH); \
			printf "\033[36m%-22s\033[0m %s\n", helpCommand,helpMessage; \
		} \
	} \
	{ lastLine = $$0 }' $(MAKEFILE_LIST)

.DEFAULT_GOAL := help
