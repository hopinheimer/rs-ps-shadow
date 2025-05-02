RUST_OUTPUT_DIR = rust-libp2p/bin/pubsub
GO_OUTPUT_DIR = go-libp2p/bin/pubsub

BINARY_NAME_RUST = rust-pubsub
BINARY_NAME_GO = go-pubsub

RUST_SOURCE_DIR = rust-libp2p
GO_SOURCE_DIR = go-libp2p
GO_SOURCE_PATH = .

all: rust-pubsub go-pubsub

rust-pubsub:
	@echo "building rust-libp2p"
	@mkdir -p $(RUST_OUTPUT_DIR)
	cd $(RUST_SOURCE_DIR) && cargo build --release
	@echo "Copying rust-libp2p pubsub binary..."
	cp $(RUST_SOURCE_DIR)/target/release/$(BINARY_NAME_RUST) $(RUST_OUTPUT_DIR)/$(BINARY_NAME_RUST)
	@echo "Finished building rust-libp2p pubsub"

go-pubsub:
	@echo "building go-libp2p"
	@mkdir -p $(GO_OUTPUT_DIR)
	cd $(GO_SOURCE_DIR) && go build -o $(GO_OUTPUT_DIR)/$(BINARY_NAME_GO) $(GO_SOURCE_PATH)
	@echo "Finished building go-libp2p pubsub"

clean:
	@echo "building built executables"
	@rm -f $(RUST_OUTPUT_DIR)/$(BINARY_NAME_RUST)
	@rm -f $(GO_OUTPUT_DIR)/$(BINARY_NAME_GO)
	@echo "Clean complete."
