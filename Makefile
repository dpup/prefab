ROOT_DIR := $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))

GEN_OUT := $(ROOT_DIR)/out
GO_SRC := $(ROOT_DIR)/

PROTO_FILES := $(shell find $(ROOT_DIR) -name "*.proto" -not -path "*/third_party/*")

TOOLS_GO := $(ROOT_DIR)/tools/tools.go
TOOLS_OUT := $(ROOT_DIR)/tools/bin
TOOL_PKGS := $(shell go list -e -f '{{join .Imports " "}}' ${TOOLS_GO})
TOOL_CMDS := $(foreach tool,$(notdir ${TOOL_PKGS}),${TOOLS_OUT}/${tool})
$(foreach cmd,${TOOL_CMDS},$(eval $(notdir ${cmd})Cmd := ${cmd}))

export PATH := $(TOOLS_OUT):$(PATH)

.PHONY: gen-proto
gen-proto: gen-proto.touchfile
gen-proto.touchfile: $(GEN_OUT)/openapiv2 $(PROTO_FILES) tools
	protoc -I$(GO_SRC) \
		-I$(ROOT_DIR)/third_party/googleapis \
		--go_out=$(GOPATH)src/ \
		--grpc_out=$(GOPATH)src/ \
		--grpc-gateway_out $(GOPATH)src \
		--grpc-gateway_opt logtostderr=true \
		--grpc-gateway_opt generate_unbound_methods=true \
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

go.mod: ${TOOLS_GO}
	@go mod tidy
	@touch go.mod

${TOOL_CMDS}: go.mod
	go build -o $@ $(filter %/$(@F),${TOOL_PKGS})

.PHONY: tools
tools: ${TOOL_CMDS}
	@# protoc looks for a different cmd name than what is installed by go.
	@cp $(TOOLS_OUT)/protoc-gen-go-grpc $(TOOLS_OUT)/protoc-gen-grpc