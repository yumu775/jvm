# 开发与贡献

## 环境

需要 Go 1.21 或更高版本。Windows 是主要交互验证平台，代码应保持 macOS/Linux 可构建。注释使用中文，错误信息和 CLI 文案保持语义明确。

```sh
go mod download
go build -trimpath -o build/ .
go test ./... -count=1 -timeout=3m
go vet ./...
```

运行开发二进制请使用完整路径，例如 `./build/jvm` 或 `.\build\jvm.exe`，避免调用旧版 PATH 中的工具。测试或开发不要求先执行 setup。

## 安全隔离

- 默认测试使用 `t.TempDir()` 和独立的 `JVM_HOME`。
- 测试不能设置真实用户/机器环境，不能编辑真实 shell profile。
- Windows 环境操作通过 `EnvironmentStore` 内存替身验证。
- HTTP 行为通过 `httptest` 验证，不依赖公网或下载真实 JDK。
- 符号链接测试在平台权限不足时可以跳过，但普通路径穿越测试必须执行。
- 网络实测只用于单独的元数据冒烟验证，不加入默认测试。
- 安装、卸载和迁移的失败路径必须验证原数据仍受保护。

## 设计约束

个人学习资料不属于应用交付内容。根目录的三份 Go/JVM 学习文档已加入 `.gitignore`；新增私人笔记统一放入忽略的 `learning/` 或 `personal-notes/` 目录。不要用 `git add -f` 将其加入提交。源码、回归测试和面向用户的 `docs/` 文档正常提交。

保持命令层编排、配置、版本定位、环境操作和下载职责清晰。所有版本定位复用 `version.Manager`，不要在新命令中重新拼默认仓库。配置修改优先使用 `config.Update`，避免多个命令覆盖彼此的修改。

新增功能需要同步 CLI help、README、CHANGELOG 和相关指南。不能仅添加成功提示来占位；未实现能力必须明确不可用。不得把环境已保存与当前 shell 已激活混为一谈。

输入路径归一化为绝对路径；删除仅针对合法托管记录，外部导入目录永远不因为取消登记被删除。shell 输出模式的 stdout 只能包含脚本，错误输出到 stderr 且返回非零。

## 检查清单

```sh
gofmt -w cmd internal main.go
go vet ./...
go test ./... -count=1 -timeout=3m
```

在支持 race detector 的环境额外执行：

```sh
go test -race ./... -timeout=3m
```

CI 的操作系统矩阵为 Windows、macOS、Linux，Go 矩阵为 1.21 与 stable。更改平台相关代码时，不应将单机通过表述为所有平台实测通过。

## 回归重点

| 范围 | 必须验证 |
| --- | --- |
| 环境 | 保留无关 PATH、注册表类型与失败回滚、临时不持久化、脚本引用安全 |
| 配置 | 旧配置默认值、绝对目录、修改仓库后旧版本可见、并发更新 |
| 安装 | 校验失败、恶意归档、旧版本保留、登记失败恢复、完整版本 ID |
| 卸载 | 外部文件保留、活动版本保护、受保护目录与路径穿越 |
| 版本 | 数字排序、完整版本和前缀、LTS 主版本去重、指定下载源 |
| 扫描 | 根目录本身、深度限制、符号链接循环、执行超时 |

## 发布

`.github/workflows/release.yml` 可手工触发或由版本标签触发，只构建并上传 Actions artifacts，不自动发布外部 Release。

标签构建使用 `vMAJOR.MINOR.PATCH`（可附预发布后缀）作为程序版本；在分支上手动构建保留源码中的开发版本，不能把 `main` 等分支名当成发布版本。

发布前确认：

1. CI 通过，补充真实 CMD/PowerShell 与 IDE 验证记录。
2. CHANGELOG 对应实际功能；升级指南描述不兼容变化。
3. 通过 `-ldflags "-X jvm/cmd.Version=<版本>"` 注入版本号。
4. 打包 Windows/Linux/macOS 的 amd64/arm64 可执行文件、安装脚本、README 和 LICENSE。
5. 校验压缩包 SHA-256，运行各平台帮助与版本命令。
6. 发布行为由维护者明确执行，不把未验证的开发构建标为稳定版。

## 报告问题

提供工具版本、系统与 shell 版本、`jvm env` 和 `jvm config list` 的相关输出、复现命令、实际与预期结果。输出可能包含用户名和本地目录，提交前可自行脱敏。

代码提交建议使用 Conventional Commits，例如 `fix(env): preserve unrelated user PATH entries`。贡献遵循 [MIT 许可证](LICENSE)。
