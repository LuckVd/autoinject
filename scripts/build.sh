#!/bin/bash
# IAST Auto Inject - Linux 平台构建脚本
# 仅支持 Linux 平台

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 项目配置
BINARY_NAME="iast-auto-inject"
VERSION="${VERSION:-1.0.0}"
BUILD_DIR="build"
DIST_DIR="dist"

# Go 路径
GO_BIN="${GO_BIN:-go}"

# Linux 支持的架构
LINUX_ARCHS="amd64 arm64"

# 打印带颜色的消息
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查依赖
check_dependencies() {
    print_info "检查依赖..."

    if ! command -v "$GO_BIN" &> /dev/null; then
        print_error "Go 未安装或不在 PATH 中"
        exit 1
    fi

    GO_VERSION=$($GO_BIN version | awk '{print $3}')
    print_success "Go 版本: $GO_VERSION"
}

# 清理构建目录
clean_build() {
    print_info "清理构建目录..."
    rm -rf "$BUILD_DIR" "$DIST_DIR"
    print_success "清理完成"
}

# 下载依赖
download_deps() {
    print_info "下载依赖..."
    $GO_BIN mod download
    $GO_BIN mod tidy
    print_success "依赖下载完成"
}

# 构建单个架构
build_arch() {
    local arch=$1
    local output_name="${BINARY_NAME}-linux-${arch}"

    print_info "构建 linux/$arch..."

    mkdir -p "$BUILD_DIR"

    GOOS=linux GOARCH=$arch $GO_BIN build \
        -ldflags="-s -w -X main.Version=${VERSION} -X main.BuildTime=$(date -u '+%Y-%m-%d_%H:%M:%S')" \
        -o "$BUILD_DIR/$output_name" .

    if [ -f "$BUILD_DIR/$output_name" ]; then
        size=$(du -h "$BUILD_DIR/$output_name" | cut -f1)
        print_success "linux/$arch 构建完成 (大小: $size)"
    else
        print_error "linux/$arch 构建失败"
        exit 1
    fi
}

# 构建所有架构
build_all() {
    print_info "开始构建 Linux 所有架构..."

    for arch in $LINUX_ARCHS; do
        build_arch "$arch"
    done

    print_success "所有架构构建完成"
}

# 仅构建 amd64
build_main() {
    print_info "构建 Linux amd64..."
    build_arch "amd64"
    print_success "Linux amd64 构建完成"
}

# 打包单个架构
package_arch() {
    local arch=$1
    local package_dir="${DIST_DIR}/${BINARY_NAME}-linux-${arch}"

    print_info "打包 linux/$arch..."

    # 创建包目录
    mkdir -p "$package_dir"

    # 复制可执行文件
    cp "$BUILD_DIR/${BINARY_NAME}-linux-${arch}" "$package_dir/${BINARY_NAME}"
    chmod +x "$package_dir/${BINARY_NAME}"

    # 复制配置文件
    cp -r configs "$package_dir/"

    # 复制安装脚本
    if [ -f "scripts/install.sh" ]; then
        cp scripts/install.sh "$package_dir/"
    fi

    # 复制 README（如果存在）
    if [ -f "README.md" ]; then
        cp README.md "$package_dir/"
    fi

    # 创建 tar.gz 压缩包
    cd "$DIST_DIR"
    tar -czf "${BINARY_NAME}-${VERSION}-linux-${arch}.tar.gz" "${BINARY_NAME}-linux-${arch}"
    cd - > /dev/null

    print_success "linux/$arch 打包完成: ${BINARY_NAME}-${VERSION}-linux-${arch}.tar.gz"
}

# 打包所有架构
package_all() {
    print_info "打包所有 Linux 架构..."

    mkdir -p "$DIST_DIR"

    for arch in $LINUX_ARCHS; do
        package_arch "$arch"
    done

    print_success "所有架构打包完成"
}

# 仅打包 amd64
package_main() {
    print_info "打包 Linux amd64..."

    mkdir -p "$DIST_DIR"

    package_arch "amd64"

    print_success "Linux amd64 打包完成"
}

# 显示构建结果
show_results() {
    echo ""
    echo "========================================"
    echo "构建摘要"
    echo "========================================"
    echo "版本: $VERSION"
    echo "平台: Linux"
    echo "构建目录: $BUILD_DIR"
    echo "发布目录: $DIST_DIR"
    echo ""

    if [ -d "$DIST_DIR" ]; then
        echo "生成的包:"
        ls -lh "$DIST_DIR"/*.tar.gz 2>/dev/null | awk '{print "  " $9 " (" $5 ")"}'
    fi

    echo ""
    echo "使用方法:"
    echo "  tar -xzf ${BINARY_NAME}-${VERSION}-linux-amd64.tar.gz"
    echo "  cd ${BINARY_NAME}-linux-amd64"
    echo "  sudo ./install.sh"
    echo ""
}

# 显示帮助
show_help() {
    cat << EOF
IAST Auto Inject - Linux 平台构建脚本

用法: $0 [选项] [命令]

命令:
  all         构建并打包所有架构
  main        构建并打包 amd64
  build       仅构建所有架构
  build-main  仅构建 amd64
  package     仅打包所有架构
  package-main 仅打包 amd64
  clean       清理构建目录
  help        显示帮助信息

选项:
  -v VERSION  指定版本号 (默认: 1.0.0)
  -g GO_BIN   指定 Go 二进制路径 (默认: go)

环境变量:
  VERSION     版本号
  GO_BIN      Go 二进制路径

示例:
  $0 all              # 构建并打包所有架构
  $0 main             # 构建并打包 amd64
  $0 -v 2.0.0 all     # 指定版本号构建
  $0 -g ~/go/bin/go main # 使用指定 Go 路径

支持的 Linux 架构: $LINUX_ARCHS

EOF
}

# 主函数
main() {
    local command=""

    # 解析参数
    while [[ $# -gt 0 ]]; do
        case $1 in
            -v|--version)
                VERSION="$2"
                shift 2
                ;;
            -g|--go)
                GO_BIN="$2"
                shift 2
                ;;
            -h|--help)
                show_help
                exit 0
                ;;
            all|main|build|build-main|package|package-main|clean)
                command="$1"
                shift
                ;;
            *)
                print_error "未知参数: $1"
                show_help
                exit 1
                ;;
        esac
    done

    # 如果没有指定命令，显示帮助
    if [ -z "$command" ]; then
        show_help
        exit 0
    fi

    echo "========================================"
    echo "IAST Auto Inject - Linux 平台构建"
    echo "========================================"
    echo "版本: $VERSION"
    echo "Go: $GO_BIN"
    echo "架构: $LINUX_ARCHS"
    echo "========================================"
    echo ""

    # 执行命令
    case "$command" in
        clean)
            clean_build
            ;;
        build)
            check_dependencies
            download_deps
            build_all
            show_results
            ;;
        build-main)
            check_dependencies
            download_deps
            build_main
            show_results
            ;;
        package)
            package_all
            show_results
            ;;
        package-main)
            package_main
            show_results
            ;;
        all)
            check_dependencies
            download_deps
            clean_build
            build_all
            package_all
            show_results
            ;;
        main)
            check_dependencies
            download_deps
            clean_build
            build_main
            package_main
            show_results
            ;;
    esac
}

# 运行主函数
main "$@"
