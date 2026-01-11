#!/bin/bash
# IAST Auto Inject - 安装脚本
# 用于 Linux 系统

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 配置
BINARY_NAME="iast-auto-inject"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/iast-inject"
USER_CONFIG_DIR="$HOME/.iast-inject"
SERVICE_DIR="/etc/systemd/system"

# 打印函数
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

# 检查是否为 root
check_root() {
    if [ "$EUID" -ne 0 ]; then
        print_warning "建议使用 sudo 运行此脚本以安装到系统目录"
        read -p "是否继续安装到用户目录? (y/N): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            print_info "请使用 sudo 运行: sudo $0"
            exit 1
        fi
        INSTALL_DIR="$HOME/.local/bin"
        CONFIG_DIR="$USER_CONFIG_DIR"
        return 1
    fi
    return 0
}

# 检测系统
detect_os() {
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        OS=$ID
        OS_VERSION=$VERSION_ID
    else
        print_error "无法检测操作系统"
        exit 1
    fi
    print_info "检测到操作系统: $OS $OS_VERSION"
}

# 查找二进制文件
find_binary() {
    # 首先检查当前目录
    if [ -f "./$BINARY_NAME" ]; then
        BINARY_PATH="./$BINARY_NAME"
        return 0
    fi

    # 检查脚本所在目录
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    if [ -f "$SCRIPT_DIR/$BINARY_NAME" ]; then
        BINARY_PATH="$SCRIPT_DIR/$BINARY_NAME"
        return 0
    fi

    print_error "找不到二进制文件 $BINARY_NAME"
    print_info "请确保在包含 $BINARY_NAME 的目录中运行此脚本"
    exit 1
}

# 安装二进制文件
install_binary() {
    print_info "安装二进制文件到 $INSTALL_DIR..."

    mkdir -p "$INSTALL_DIR"

    if cp "$BINARY_PATH" "$INSTALL_DIR/$BINARY_NAME"; then
        chmod +x "$INSTALL_DIR/$BINARY_NAME"
        print_success "二进制文件安装完成"
    else
        print_error "二进制文件安装失败"
        exit 1
    fi
}

# 安装配置文件
install_config() {
    print_info "安装配置文件..."

    # 检查配置目录
    if [ -d "./configs" ]; then
        CONFIG_SOURCE="./configs"
    elif [ -d "$SCRIPT_DIR/configs" ]; then
        CONFIG_SOURCE="$SCRIPT_DIR/configs"
    else
        print_warning "找不到配置文件目录，跳过配置文件安装"
        return 0
    fi

    # 系统配置
    if [ "$EUID" -eq 0 ]; then
        mkdir -p "$CONFIG_DIR"
        cp -r "$CONFIG_SOURCE"/* "$CONFIG_DIR/"
        print_success "系统配置安装到 $CONFIG_DIR"
    else
        mkdir -p "$USER_CONFIG_DIR"
        cp -r "$CONFIG_SOURCE"/* "$USER_CONFIG_DIR/"
        print_success "用户配置安装到 $USER_CONFIG_DIR"
    fi
}

# 创建 systemd 服务（仅 root）
install_service() {
    if [ "$EUID" -ne 0 ]; then
        print_warning "需要 root 权限才能安装 systemd 服务"
        return 0
    fi

    print_info "创建 systemd 服务..."

    cat > "$SERVICE_DIR/$BINARY_NAME.service" << EOF
[Unit]
Description=IAST Auto Inject Daemon
After=network.target

[Service]
Type=simple
User=root
ExecStart=$INSTALL_DIR/$BINARY_NAME daemon --config $CONFIG_DIR/config.yaml
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    print_success "systemd 服务创建完成"
    print_info "使用以下命令管理服务:"
    echo "  启动:   sudo systemctl start $BINARY_NAME"
    echo "  停止:   sudo systemctl stop $BINARY_NAME"
    echo "  状态:   sudo systemctl status $BINARY_NAME"
    echo "  开机自启: sudo systemctl enable $BINARY_NAME"
}

# 设置环境变量
setup_env() {
    if [ "$EUID" -ne 0 ]; then
        # 检查 .bashrc 是否包含 INSTALL_DIR
        if ! grep -q "$INSTALL_DIR" "$HOME/.bashrc" 2>/dev/null; then
            print_info "添加 $INSTALL_DIR 到 PATH"
            echo "export PATH=\"$INSTALL_DIR:\$PATH\"" >> "$HOME/.bashrc"
            print_warning "请运行 'source ~/.bashrc' 或重新登录以更新 PATH"
        fi
    fi
}

# 显示安装信息
show_info() {
    echo ""
    echo "========================================"
    echo "安装完成"
    echo "========================================"
    echo "二进制文件: $INSTALL_DIR/$BINARY_NAME"
    if [ "$EUID" -eq 0 ]; then
        echo "配置文件:   $CONFIG_DIR"
        echo "服务文件:   $SERVICE_DIR/$BINARY_NAME.service"
    else
        echo "配置文件:   $USER_CONFIG_DIR"
    fi
    echo ""
    echo "快速开始:"
    echo "  1. 查看帮助:     $BINARY_NAME --help"
    echo "  2. 列出进程:    $BINARY_NAME list"
    echo "  3. 查看配置:    $BINARY_NAME config show"
    echo ""
    if [ "$EUID" -eq 0 ]; then
        echo "启用守护进程:"
        echo "  sudo systemctl enable $BINARY_NAME"
        echo "  sudo systemctl start $BINARY_NAME"
    fi
    echo ""
}

# 卸载
uninstall() {
    print_info "卸载 $BINARY_NAME..."

    if [ "$EUID" -eq 0 ]; then
        # 停止并禁用服务
        if systemctl is-active --quiet "$BINARY_NAME" 2>/dev/null; then
            print_info "停止服务..."
            systemctl stop "$BINARY_NAME"
        fi

        if systemctl is-enabled --quiet "$BINARY_NAME" 2>/dev/null; then
            print_info "禁用服务..."
            systemctl disable "$BINARY_NAME"
        fi

        # 删除文件
        rm -f "$INSTALL_DIR/$BINARY_NAME"
        rm -f "$SERVICE_DIR/$BINARY_NAME.service"
        rm -rf "$CONFIG_DIR"

        systemctl daemon-reload
        print_success "卸载完成"
    else
        # 用户卸载
        rm -f "$INSTALL_DIR/$BINARY_NAME"
        rm -rf "$USER_CONFIG_DIR"
        print_success "用户安装已卸载"
    fi
}

# 显示帮助
show_help() {
    cat << EOF
IAST Auto Inject - 安装脚本

用法: $0 [选项]

选项:
  install    安装（默认）
  uninstall  卸载
  help       显示帮助信息

环境变量:
  BINARY_NAME  二进制文件名（默认: iast-auto-inject）
  INSTALL_DIR  安装目录（默认: /usr/local/bin 或 ~/.local/bin）

示例:
  sudo $0           # 安装到系统目录
  $0                # 安装到用户目录
  sudo $0 uninstall # 卸载

EOF
}

# 主函数
main() {
    local command="${1:-install}"

    case "$command" in
        install)
            echo "========================================"
            echo "IAST Auto Inject - 安装向导"
            echo "========================================"
            echo ""

            detect_os
            check_root
            find_binary
            install_binary
            install_config
            install_service
            setup_env
            show_info
            ;;
        uninstall)
            uninstall
            ;;
        help|--help|-h)
            show_help
            ;;
        *)
            print_error "未知命令: $command"
            show_help
            exit 1
            ;;
    esac
}

# 运行主函数
main "$@"
