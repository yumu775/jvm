#!/bin/bash

# JVM Tool 快速安装脚本 (Linux/macOS)
# 这个脚本会自动配置 JVM 工具的环境变量

set -e

# 默认配置
INSTALL_PATH="$HOME/bin"
FORCE=false
SYSTEM_WIDE=false

# 解析命令行参数
while [[ $# -gt 0 ]]; do
    case $1 in
        --install-path)
            INSTALL_PATH="$2"
            shift 2
            ;;
        --force)
            FORCE=true
            shift
            ;;
        --system-wide)
            SYSTEM_WIDE=true
            shift
            ;;
        --help)
            echo "JVM Tool Quick Setup"
            echo ""
            echo "Usage: $0 [options]"
            echo ""
            echo "Options:"
            echo "  --install-path PATH   Install to specific path (default: $HOME/bin)"
            echo "  --force              Force reinstall"
            echo "  --system-wide        System-wide installation (requires sudo)"
            echo "  --help               Show this help"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

echo "=== JVM Tool Quick Setup ==="
echo ""

# 检查 JVM 可执行文件
JVM_EXE="./jvm"
if [[ ! -f "$JVM_EXE" ]]; then
    echo "Error: jvm executable not found in current directory"
    echo "Please run this script from the directory containing the jvm binary"
    exit 1
fi

echo "Found JVM tool: $(pwd)/jvm"

# 检查是否已经在 PATH 中
CURRENT_DIR=$(pwd)
if echo "$PATH" | grep -q "$CURRENT_DIR" && [[ "$FORCE" != true ]]; then
    echo "JVM tool is already in PATH!"
    echo "Current directory: $CURRENT_DIR"
    echo ""
    echo "Test with: jvm --version"
    exit 0
fi

# 选择安装方式
echo "Choose installation method:"
echo "1. Add current directory to PATH (recommended)"
echo "2. Copy to $INSTALL_PATH and add to PATH"
echo "3. Use JVM's built-in setup command"
echo ""

read -p "Enter choice (1-3): " choice

case $choice in
    1)
        # 方式1：直接添加当前目录到 PATH
        echo "Adding current directory to PATH..."
        
        # 使用 JVM 的内置 setup 命令
        if $JVM_EXE setup --force; then
            echo "Setup completed successfully!"
        else
            echo "Setup failed. Trying manual configuration..."
            
            # 手动配置 shell profile
            SHELL_NAME=$(basename "$SHELL")
            case $SHELL_NAME in
                bash)
                    PROFILE_FILE="$HOME/.bashrc"
                    if [[ -f "$HOME/.bash_profile" ]]; then
                        PROFILE_FILE="$HOME/.bash_profile"
                    fi
                    ;;
                zsh)
                    PROFILE_FILE="$HOME/.zshrc"
                    ;;
                fish)
                    PROFILE_FILE="$HOME/.config/fish/config.fish"
                    mkdir -p "$(dirname "$PROFILE_FILE")"
                    ;;
                *)
                    PROFILE_FILE="$HOME/.profile"
                    ;;
            esac
            
            echo "Configuring $PROFILE_FILE..."
            
            # 检查是否已经配置
            if ! grep -q "JVM Tool PATH" "$PROFILE_FILE" 2>/dev/null; then
                echo "" >> "$PROFILE_FILE"
                echo "# JVM Tool PATH Configuration" >> "$PROFILE_FILE"
                echo "export PATH=\"$CURRENT_DIR:\$PATH\"" >> "$PROFILE_FILE"
                echo "" >> "$PROFILE_FILE"
                echo "Added to shell profile: $PROFILE_FILE"
            else
                echo "Already configured in: $PROFILE_FILE"
            fi
        fi
        ;;
        
    2)
        # 方式2：复制到指定目录
        echo "Installing to: $INSTALL_PATH"
        
        # 创建目录
        mkdir -p "$INSTALL_PATH"
        
        # 复制可执行文件
        TARGET_EXE="$INSTALL_PATH/jvm"
        cp "$JVM_EXE" "$TARGET_EXE"
        chmod +x "$TARGET_EXE"
        echo "Copied jvm to: $TARGET_EXE"
        
        # 使用复制后的 JVM 进行 setup
        if "$TARGET_EXE" setup --force; then
            echo "Setup completed successfully!"
        else
            echo "Setup failed. Please check the installation."
        fi
        ;;
        
    3)
        # 方式3：使用内置 setup 命令
        echo "Running JVM setup command..."
        if $JVM_EXE setup --force; then
            echo "Setup completed successfully!"
        else
            echo "Setup failed. Please try manual configuration."
            exit 1
        fi
        ;;
        
    *)
        echo "Invalid choice. Exiting."
        exit 1
        ;;
esac

echo ""
echo "=== Next Steps ==="
echo "1. Restart your terminal or run: source ~/.bashrc (or your shell's config file)"
echo "2. Test with: jvm --version"
echo "3. Start using: jvm list"
echo ""
echo "If you encounter issues, try:"
echo "  jvm setup --help"
echo "  jvm platform info"
