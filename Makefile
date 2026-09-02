BINARY_NAME=tu
TERMUX_DIR?=/data/data/com.termux/files
INSTALL_DIR?=$(TERMUX_DIR)/home/.local/bin

# Detect current shell or default to bash
SHELL_TYPE ?= bash

.PHONY: build
build:
	go build -o bin/$(BINARY_NAME) main.go

.PHONY: install
install: build
	# Create folder if not exists
	mkdir -p $(INSTALL_DIR)
	# Move binary to path (requires sudo if writing to /usr/local/bin)
	cp bin/$(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)

.PHONY: completion
completion: install
	@echo "Generating and installing completion for $(SHELL_TYPE)..."
	@if [ "$(SHELL_TYPE)" = "bash" ]; then \
		$(BINARY_NAME) completion bash | tee $(TERMUX_DIR)/usr/etc/bash_completion.d/$(BINARY_NAME) > /dev/null; \
	# elif [ "$(SHELL_TYPE)" = "zsh" ]; then \
	#	mkdir -p ~/.zsh/completions; \
	#	$(BINARY_NAME) completion zsh > ~/.zsh/completions/_$(BINARY_NAME); \
	# elif [ "$(SHELL_TYPE)" = "fish" ]; then \
	#	mkdir -p ~/.config/fish/completions; \
	#	$(BINARY_NAME) completion fish > ~/.config/fish/completions/$(BINARY_NAME).fish; \
	fi
	@echo "Completion installed successfully! Please restart your shell or source your profile."
