OUT = gbc
GTEST = gtest

GO = go
#ifneq ($(shell command -v gore 2>/dev/null),)
#GO = gore
#endif

# `gore` is my wrapper for `go` that adds beautiful errors like this one:
#     # $ gore build
#     # gbc/pkg/codegen
#       71 | // NewContext creates a new code generation context.
#       72 | func NewContext(targetArch string, cfg *config.Config) *Context {
#       73 |     invalidSyntaxToShowcaseGore
#       -- |     ^~~~~~~~~~~~~~~~~~~~~~~~~~~  undefined: invalidSyntaxToShowcaseGore
#       74 |     return &Context{
#       75 |         currentScope: newScope(nil),
#
#       83 |
#       84 | func newScope(parent *scope) *scope {
#       85 |     hellooooo
#       -- |     ^~~~~~~~~  undefined: hellooooo
#       86 |     return &scope{Parent: parent}
#       87 | }
#
# I might publish `gore` at some point...
# NOTE: `gore` uses syntax highlighting and pretty colors, this showcase doesn't do it justice
#

all: $(OUT)

$(OUT):
	@echo "Building gbc..."
	@$(GO) build -C ./cmd/$(OUT) -o ../../$(OUT)

$(GTEST):
	@echo "Building test runner..."
	@$(GO) build -C ./cmd/gtest #-o ../../$(GTEST)

clean:
	@echo "Cleaning up..."
	@rm -f $(OUT) $(GTEST) ./gbc ./cmd/gtest/gtest ./a.out ./.test_results.json

ARCH := $(shell uname -m)
OS := $(shell uname -s)
LIBB := ./lib/b/$(ARCH)-$(OS).b

badFiles := raylib.b donut.b

define filter_files
files=""; \
for f in $(1); do \
  skip=0; \
  for bad in $(badFiles); do \
    if [ "$$f" = "$(2)/$$bad" ]; then skip=1; break; fi; \
  done; \
  if [ $$skip -eq 0 ]; then files="$$files $$f"; fi; \
done; \
echo "$$files"
endef

test: all $(GTEST)
	@echo "Running tests..."
	@files=$$( $(call filter_files,tests/*.b*,tests) ); \
	./cmd/$(GTEST)/$(GTEST) --test-files="$$files" --target-args="$(GBCFLAGS) $(LIBB)" -v --ignore-lines="addresses"

examples: all $(GTEST)
	@echo "Running examples..."
	@files=$$( $(call filter_files,examples/*.b*,examples) ); \
	./cmd/$(GTEST)/$(GTEST) --test-files="$$files" --target-args="$(GBCFLAGS) $(LIBB)" -v --ignore-lines="xs_items"
