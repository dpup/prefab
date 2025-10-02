ROOT_DIR := $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))

GEN_OUT := $(ROOT_DIR)/out

PROTO_FILES := $(shell find $(ROOT_DIR) -name "*.proto" -not -path "*/third_party/*")

TOOLS_OUT := $(ROOT_DIR)/tools/bin
TOOL_PKGS := $(shell go tool | grep -E '^(github.com|google.golang.org)')
TOOL_CMDS := $(foreach tool,$(notdir ${TOOL_PKGS}),${TOOLS_OUT}/${tool})
$(foreach cmd,${TOOL_CMDS},$(eval $(notdir ${cmd})Cmd := ${cmd}))

export PATH := $(TOOLS_OUT):$(PATH)

.PHONY: lint
lint:
	@golangci-lint run

.PHONY: fix
fix:
	@echo "🔧 Running golangci-lint with --fix..."
	@golangci-lint run --fix
	@echo "✅ Auto-fixes applied"

.PHONY: test
test: test-staticcheck test-vet test-unit

test-unit: gen-proto
	@go test ./... -cover

test-vet:
	@go vet ./...

test-staticcheck:
	@staticcheck ./...

.PHONY: test-coverage
test-coverage: gen-proto
	@TARGET_PATH="$(if $(TARGET),$(TARGET),./...)"; \
	if [ "$(PORCELAIN)" = "1" ]; then \
		echo "🧪 Generating test coverage report..."; \
		go test $$TARGET_PATH -coverprofile cover.out.tmp; \
		cat cover.out.tmp | grep -v ".pb.go" | grep -v ".pb.gw.go" > cover.out; \
		rm cover.out.tmp; \
		go tool cover -func cover.out; \
		echo ""; \
		echo "📊 Full coverage report saved to cover.out"; \
		echo "💡 View HTML report with: go tool cover -html=cover.out"; \
	else \
		echo "🧪 Running tests with coverage..."; \
		echo ""; \
		go test $$TARGET_PATH -coverprofile cover.out.tmp; \
		cat cover.out.tmp | grep -v ".pb.go" | grep -v ".pb.gw.go" > cover.out; \
		rm cover.out.tmp; \
		echo ""; \
		echo "📊 Coverage Summary (excluding generated files):"; \
		echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"; \
		go tool cover -func cover.out | tail -1; \
		echo ""; \
		echo "💾 Full report saved to cover.out"; \
		echo "💡 View detailed HTML report: go tool cover -html=cover.out"; \
		echo "🤖 Machine-readable output: make test-coverage PORCELAIN=1"; \
	fi

.PHONY: clean-proto
clean-proto:
	@rm -f gen-proto.touchfile
	@find . -name "*.pb.go" -type f -delete
	@find . -name "*.pb.gw.go" -type  f -delete
	@echo "👷🏽‍♀️ Generated proto files removed"

.PHONY: clean-coverage
clean-coverage:
	@rm -f cover.out cover.out.tmp
	@echo "🧹 Coverage files removed"

.PHONY: gen-proto
gen-proto: gen-proto.touchfile
gen-proto.touchfile: $(GEN_OUT)/openapiv2 $(PROTO_FILES) tools.touchfile
	@protoc -I$(ROOT_DIR)/proto \
		-I$(ROOT_DIR)/proto/third_party/googleapis \
		--go_out=$(GOPATH)src/ \
		--grpc_out=$(GOPATH)src/ \
		--grpc-gateway_out $(GOPATH)src \
		--grpc-gateway_opt logtostderr=true \
		--grpc-gateway_opt generate_unbound_methods=false \
		--grpc-gateway_opt omit_package_doc=true \
		--openapiv2_out $(GEN_OUT)/openapiv2 \
		--openapiv2_opt logtostderr=true \
		--openapiv2_opt use_go_templates=true \
		--openapiv2_opt simple_operation_ids=true \
		--openapiv2_opt openapi_naming_strategy=fqn \
		--openapiv2_opt disable_default_errors=true \
		--openapiv2_opt allow_merge=true \
		$(PROTO_FILES)
	@touch gen-proto.touchfile
	@echo "👷🏽‍♀️ Protos generated"

$(GEN_OUT)/openapiv2:
	@mkdir -p $(GEN_OUT)/openapiv2

go.mod:
	@go mod tidy
	@touch go.mod

${TOOL_CMDS}: go.mod
	@mkdir -p $(TOOLS_OUT)
	@go build -o $@ $(filter %/$(@F),${TOOL_PKGS})

.PHONY: tools
tools: tools.touchfile 
tools.touchfile: ${TOOL_CMDS}
	@# protoc looks for a different cmd name than what is installed by go.
	@touch tools.touchfile
	@cp $(TOOLS_OUT)/protoc-gen-go-grpc $(TOOLS_OUT)/protoc-gen-grpc