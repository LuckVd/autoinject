# Makefile for IAST Auto Inject (Linux Only)
# Linux 平台专用构建和打包

# 项目信息
BINARY_NAME=iast-auto-inject
VERSION?=1.0.0
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Go 编译参数
GO=go
GOFLAGS=-v -ldflags="-s -w -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)"

# 构建目录
BUILD_DIR=build
DIST_DIR=dist

# 支持的 Linux 架构
LINUX_ARCHS=amd64 arm64

# 默认目标
.PHONY: all
all: clean build

# 清理构建目录
.PHONY: clean
clean:
	@echo "Cleaning build directories..."
	@rm -rf $(BUILD_DIR) $(DIST_DIR)
	@echo "Clean completed"

# 下载依赖
.PHONY: deps
deps:
	@echo "Downloading dependencies..."
	@$(GO) mod download
	@$(GO) mod tidy
	@echo "Dependencies downloaded"

# 格式化代码
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	@$(GO) fmt ./...
	@echo "Code formatted"

# 代码检查
.PHONY: vet
vet:
	@echo "Vetting code..."
	@$(GO) vet ./...
	@echo "Code vetted"

# 运行测试
.PHONY: test
test:
	@echo "Running tests..."
	@$(GO) test -v ./...

# 本地构建
.PHONY: build
build: deps fmt vet
	@echo "Building $(BINARY_NAME) for Linux..."
	@mkdir -p $(BUILD_DIR)
	@$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) .
	@echo "Build completed: $(BUILD_DIR)/$(BINARY_NAME)"

# 构建 Linux 所有架构
.PHONY: build-all
build-all: deps fmt vet
	@echo "Building for all Linux architectures..."
	@mkdir -p $(BUILD_DIR)
	@$(foreach arch,$(LINUX_ARCHS),\
		echo "Building linux/$(arch)..."; \
		GOOS=linux GOARCH=$(arch) $(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-$(arch)) .; \
	)
	@echo "All Linux builds completed"

# 打包所有架构
.PHONY: package-all
package-all: build-all
	@echo "Packaging all Linux architectures..."
	@mkdir -p $(DIST_DIR)
	@$(foreach arch,$(LINUX_ARCHS),\
		echo "Packaging linux/$(arch)..."; \
		mkdir -p $(DIST_DIR)/$(BINARY_NAME)-linux-$(arch); \
		cp $(BUILD_DIR)/$(BINARY_NAME)-linux-$(arch) $(DIST_DIR)/$(BINARY_NAME)-linux-$(arch)/$(BINARY_NAME); \
		chmod +x $(DIST_DIR)/$(BINARY_NAME)-linux-$(arch)/$(BINARY_NAME); \
		cp -r configs $(DIST_DIR)/$(BINARY_NAME)-linux-$(arch)/; \
		cp scripts/install.sh $(DIST_DIR)/$(BINARY_NAME)-linux-$(arch)/; \
		cd $(DIST_DIR); \
		tar -czf $(BINARY_NAME)-$(VERSION)-linux-$(arch).tar.gz $(BINARY_NAME)-linux-$(arch); \
		cd -; \
	)
	@echo "Packaging completed"

# 快速打包（仅 amd64）
.PHONY: package
package: build
	@echo "Packaging Linux amd64..."
	@mkdir -p $(DIST_DIR)/$(BINARY_NAME)-linux-amd64
	@cp $(BUILD_DIR)/$(BINARY_NAME) $(DIST_DIR)/$(BINARY_NAME)-linux-amd64/
	@chmod +x $(DIST_DIR)/$(BINARY_NAME)-linux-amd64/$(BINARY_NAME)
	@cp -r configs $(DIST_DIR)/$(BINARY_NAME)-linux-amd64/
	@cp scripts/install.sh $(DIST_DIR)/$(BINARY_NAME)-linux-amd64/
	@cd $(DIST_DIR) && tar -czf $(BINARY_NAME)-$(VERSION)-linux-amd64.tar.gz $(BINARY_NAME)-linux-amd64 && cd ..
	@echo "Packaging completed"

# 安装到系统
.PHONY: install
install: build
	@echo "Installing $(BINARY_NAME)..."
	@sudo mkdir -p /usr/local/bin
	@sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/
	@sudo chmod +x /usr/local/bin/$(BINARY_NAME)
	@mkdir -p ~/.iast-inject
	@cp -r configs ~/.iast-inject/ 2>/dev/null || true
	@echo "Installation completed"
	@echo "Run 'iast-auto-inject --help' to get started"

# 卸载
.PHONY: uninstall
uninstall:
	@echo "Uninstalling $(BINARY_NAME)..."
	@sudo rm -f /usr/local/bin/$(BINARY_NAME)
	@echo "Uninstallation completed"

# 运行
.PHONY: run
run: build
	@./$(BUILD_DIR)/$(BINARY_NAME) $(ARGS)

# 帮助信息
.PHONY: help
help:
	@echo "IAST Auto Inject - Makefile (Linux Only)"
	@echo ""
	@echo "Usage:"
	@echo "  make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  all         - 清理并构建（默认）"
	@echo "  clean       - 清理构建目录"
	@echo "  deps        - 下载依赖"
	@echo "  fmt         - 格式化代码"
	@echo "  vet         - 代码检查"
	@echo "  test        - 运行测试"
	@echo "  build       - 本地构建 (当前架构)"
	@echo "  build-all   - 构建 Linux 所有架构"
	@echo "  package     - 打包 Linux amd64"
	@echo "  package-all - 打包 Linux 所有架构"
	@echo "  install     - 安装到系统"
	@echo "  uninstall   - 从系统卸载"
	@echo "  run         - 运行程序（ARGS='参数）"
	@echo "  help        - 显示帮助信息"
	@echo ""
	@echo "Examples:"
	@echo "  make                    # 构建"
	@echo "  make package            # 打包 amd64"
	@echo "  make package-all        # 打包所有架构"
	@echo "  make install            # 安装到系统"
	@echo "  make run ARGS='list'    # 运行 list 命令"
	@echo ""
	@echo "Supported Linux Architectures: $(LINUX_ARCHS)"
