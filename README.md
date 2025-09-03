# JVM - Java 版本管理工具

JVM 是一个功能强大、简单易用的 Java 版本管理工具，灵感来自 nvm（Node Version Manager）。它允许你轻松地安装、管理和切换不同版本的 Java。

## ✨ 主要特性

- 🚀 **简单易用**: 类似 nvm 的命令行界面
- 📦 **多源下载**: 支持 Eclipse Adoptium、Amazon Corretto、Azul Zulu、Oracle JDK、GraalVM
- 🔄 **快速切换**: 在不同 Java 版本间快速切换
- 🏷️ **智能别名**: 支持 `latest`、`lts`、`stable` 等语义化版本
- 🎯 **项目配置**: 支持项目级的 Java 版本配置 (`.jvmrc`)
- 🔍 **智能扫描**: 自动发现和导入系统中已安装的 Java 版本
- 🗑️ **安全卸载**: 完整的版本卸载功能，支持预览模式
- 🌍 **跨平台**: 支持 Windows、macOS 和 Linux
- ⚡ **一键安装**: 自动配置环境变量，直接使用 `jvm` 命令

## 安装

### 前置要求

- Go 1.21 或更高版本

### 从源码构建

```bash
# 克隆项目
git clone https://github.com/yumu775/jvm.git
cd jvm

# 下载依赖
go mod tidy

# 构建
go build -o jvm

# 安装到系统路径（可选）
go install
```

## 🚀 快速开始

### 一键环境配置

构建完成后，首先配置环境变量，让你可以在任何地方直接使用 `jvm` 命令：

#### Windows 用户

**方法 1: 双击运行（推荐）**

```cmd
# 双击 setup-jvm.bat 文件
setup-jvm.bat
```

**方法 2: PowerShell 脚本**

```powershell
# 在 PowerShell 中运行
powershell -ExecutionPolicy Bypass -File quick-setup.ps1
```

**方法 3: 使用内置命令**

```cmd
# 在当前目录运行
.\jvm.exe setup
```

#### Linux/macOS 用户

```bash
# 给脚本执行权限并运行
chmod +x install-jvm.sh
./install-jvm.sh

# 或使用内置命令
./jvm setup
```

## 使用方法

### 基本命令

配置完环境变量后，重启终端，然后就可以直接使用 `jvm` 命令：

```bash
# 查看帮助
jvm --help

# 列出已安装的 Java 版本
jvm list

# 列出系统中的 Java 版本
jvm list --system

# 安装 Java 版本（支持别名）
jvm install 17              # 安装 Java 17
jvm install latest          # 安装最新版本
jvm install lts             # 安装最新 LTS 版本
jvm install 17 --source corretto  # 从 Amazon Corretto 安装

# 切换到指定版本
jvm use 17

# 查看当前版本
jvm current

# 扫描系统中已安装的 Java
jvm scan
jvm scan --path E:\Java     # 扫描指定路径

# 导入系统中的 Java 版本
jvm import 11
jvm import --from E:\JavaVersions  # 从指定路径导入
jvm import --all --from E:\Java     # 导入指定路径下的所有版本

# 卸载 Java 版本
jvm uninstall 11 --dry-run  # 预览卸载
jvm uninstall 11            # 实际卸载

# 管理配置
jvm config add-path E:\JavaVersions  # 添加自定义扫描路径
jvm config list                      # 查看所有配置

# 版本别名
jvm alias list              # 查看所有别名
jvm alias resolve lts       # 解析别名

# 下载源管理
jvm sources list            # 查看下载源

# 直接设置环境变量
jvm set-env E:\Java\jdk-17  # 直接使用指定路径的 Java

# 平台信息
jvm platform info           # 查看平台支持信息
```

### 示例工作流

```bash
# 1. 安装 Java 17
jvm install 17

# 2. 切换到 Java 17
jvm use 17

# 3. 查看当前版本
jvm current

# 4. 列出所有已安装版本
jvm list
```

## 项目结构

```
jvm/
├── main.go                 # 程序入口点
├── go.mod                  # Go 模块定义
├── cmd/                    # 命令行相关代码
│   ├── root.go            # 根命令定义
│   ├── list.go            # list 命令
│   ├── current.go         # current 命令
│   ├── use.go             # use 命令
│   └── install.go         # install 命令
├── internal/               # 内部包（不对外暴露）
│   ├── config/            # 配置管理
│   │   └── config.go      # 配置结构和操作
│   └── version/           # 版本管理
│       └── manager.go     # 版本管理器
└── README.md              # 项目说明
```

## Go 语言学习要点

### 1. 包（Package）系统

- `package main`: 可执行程序的入口包
- `package cmd`: 命令行相关功能
- `internal/`: 内部包，不能被外部项目导入

### 2. 错误处理

```go
// Go 的错误处理模式
result, err := someFunction()
if err != nil {
    return fmt.Errorf("operation failed: %w", err)
}
```

### 3. 结构体和方法

```go
// 定义结构体
type Manager struct {
    config *config.Config
}

// 为结构体定义方法
func (m *Manager) GetCurrent() (string, error) {
    // 方法实现
}
```

### 4. 接口和依赖注入

```go
// 通过构造函数模式创建实例
func NewManager() (*Manager, error) {
    // 初始化逻辑
}
```

### 5. 并发和 Goroutines

```go
// 启动 goroutine
go func() {
    // 并发执行的代码
}()
```

## 配置文件

工具会在用户主目录下创建 `.jvm` 目录：

```
~/.jvm/
├── config.json           # 配置文件
└── versions/             # Java 版本安装目录
    ├── java-17/         # Java 17 安装
    └── java-11.0.19/    # Java 11.0.19 安装
```

## 开发计划

- [x] 基础项目结构
- [x] 命令行界面
- [x] 版本管理功能
- [x] 配置管理
- [ ] 真实的 Java 下载功能
- [ ] 自动环境变量设置
- [ ] Shell 集成脚本
- [ ] 项目级配置（.jvmrc）
- [ ] 更多下载源支持

## 贡献

欢迎提交 Issue 和 Pull Request！

## 许可证

MIT License
