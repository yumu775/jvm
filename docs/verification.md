# 本次实现验证记录

验证环境：Windows，2026-09-30。此记录针对 1.1.0-dev 源码与本地构建，不代表已发布稳定版本。

## 已通过

| 检查 | 结果 |
| --- | --- |
| `go test ./... -count=1 -timeout=3m` | 全部包通过，包括独立 CLI 子进程与隔离 PowerShell 集成测试 |
| `go vet ./...` | 通过 |
| `gofmt` / `git diff --check` | 通过；Git 仅提示 Windows 脚本行尾转换 |
| Windows amd64 构建 | `build/jvm.exe`，`--version` 返回 `1.1.0-dev` |
| Linux amd64 交叉构建 | `build/jvm-linux-amd64` |
| macOS ARM64 交叉构建 | `build/jvm-darwin-arm64` |
| Windows ARM64 交叉构建 | `build/jvm-windows-arm64.exe` |
| CLI 帮助与激活接口 | 新版帮助、版本和激活脚本输出检查通过 |
| Adoptium 公开元数据 | Windows package 为 ZIP、独立 installer 为 MSI；macOS ARM64 package 为 tar.gz；真实带 `.LTS` 构建版本可解析 |
| 多来源目录 | Adoptium、Zulu、Corretto、GraalVM 查询与 `sources check` 公网验证通过；全主版本摘要约 6.7 秒（单次观测，不是性能保证） |
| 完整 JDK 安装 | Zulu `17.0.20.1+1`，下载约 186.8 MiB ZIP，通过 SHA-256，安装到 `E:/jvm/build/smoke-jdks`，`java -version` 与 `javac -version` 均运行成功 |
| 离线与已安装标记 | 隔离 `JVM_HOME=E:/jvm/build/catalog-smoke` 下，`list` 显示真实自定义目录，`list available 17 --source zulu --offline` 标记 `INSTALLED=yes` |

## 自动回归覆盖

- 临时激活不写配置，stdout 只包含脚本，失败返回非零。
- PowerShell 集成在调用会话生效，路径中的单引号和 `$` 按字面处理。
- 持久环境失败恢复、PATH 类型和无关目录保留、匹配环境清理。
- 安装暂存替换、登记失败回滚、旧目录与普通文件保护。
- 自定义目录可见、旧仓库兼容、并发配置更新、路径边界。
- 外部文件保留、嵌套登记保护、解除登记后不被旧扫描重新认领。
- 安装与卸载互斥、活动版本与默认引用清理。
- HTTP 元数据校验、版本别名、缓存校验、恶意归档、macOS 布局。
- 扫描根目录、超时与 release 元数据；项目配置格式与错误处理。
- 列表摘要、完整历史、纯 JSON、厂商准确匹配、别名共享目录与默认来源。
- 分来源缓存、离线、强制刷新、过期上限、部分来源失败和同厂商备用接口。
- 下载瞬时失败重试、校验失败拒绝、多厂商版本共存及简写歧义检查。
- `uninstall clean --dry-run` 不改配置；实际清理保留权限不明、正在安装和仍存在的目录。

实际安装发现 Azul API 的 `size=195862000` 是约整值，CDN 实际文件为 `195861965` 字节；已将其保留为参考元数据，不再用于精确长度判断。传输错误检查与 SHA-256 保持生效，旧缓存兼容及错误校验和均有回归测试。

## 尚未进行的实机验收

- 2026-09-30 未调用真实 setup/use；2026-10-01 已按授权执行工具 setup，详见下节。仍未切换真实 Java 选择或修改现有 JDK。
- 完整公网安装目前只实测 Zulu Windows amd64；其他来源已验证元数据/校验信息，并使用本地 HTTP 与归档夹具覆盖安装行为，未逐一下载全部厂商 JDK。
- 未运行用户的 Android Studio/Gradle 项目。IDE 的 Gradle JDK 与构建兼容性需按[排查手册](troubleshooting.md)核对。
- Linux/macOS/Windows ARM64 本地仅交叉构建，未运行对应二进制。
- 已添加跨平台 CI 与 Linux race 检查配置，尚未在远程 Actions 执行。

上述 2026-09-30 验证使用 `build/jvm.exe`，当时没有自动替换仓库根目录的旧程序。

## 2026-10-01：Windows 构建与启动流程补充

新增 `build.ps1`：先编译到临时文件，成功后发布 `build/jvm.exe` 并替换根目录 `jvm.exe`；编译失败保留已有程序。替换根目录程序失败时保留成功构建的 `build/jvm.exe` 并报错，便于关闭占用程序后重试。

默认构建不写用户环境；仅显式传入 `-Setup` 时调用 `quick-setup.ps1`。README 已将 Windows 源码构建和快速开始统一为该流程，解释双击旧命令行程序的 Cobra 提示。

本轮实际验证结果：

- `go test ./... -count=1 -timeout=3m`、`go vet ./...`、格式检查通过；新增 Windows 启动确认、profile 编码/内容保护、可执行文件覆盖保护和隔离安装脚本测试。
- 实际执行 `build.ps1` 成功更新两处程序，SHA-256 相同，均为 `1.1.0-dev`。首次运行发现 PowerShell 5 将 .NET `File.Replace` 的 `$null` 备份路径绑定成空字符串，已改为 `[NullString]::Value` 并重新验证成功，失败时旧程序保留。
- 实际执行 `quick-setup.ps1`，将 `E:\jvm` 加入真实用户 PATH，并写入 Windows PowerShell 的用户级 AllHosts profile；当前会话验证成功。
- 从不含 `jvm.exe` 的目录启动新 PowerShell，自动加载 `jvm` 函数并返回新版号。
- 在真实用户环境中根据持久机器/用户 PATH 创建 CMD，`where jvm` 定位到 `E:\jvm\jvm.exe`，`jvm --version` 成功。沙箱隔离的用户注册表不是该实机验收结果。
- 未实际自动化双击 Explorer GUI；双击确认分支有单元测试，安装脚本有隔离集成测试。

本轮只更新和配置 JVM 工具入口，没有执行真实 `jvm use` 更换用户 Java。Windows PowerShell 与 PowerShell 7 各自使用不同 profile，本机验收覆盖前者。

## 提交前质量审查

本次按 CLI 应用交付标准检查核心行为、失败保护、并发、安装升级、文档和发布范围。结论是可提交的开发版本，不能替代完整稳定版发布验收。

| 项目 | 结果 |
|---|---|
| 全量回归与静态检查 | `go test ./... -count=1 -timeout=3m`、`go vet ./...` 通过 |
| 最终代码竞态回归 | Windows `go test -race ./... -count=1 -timeout=3m` 全部通过 |
| 活动 JDK 卸载保护 | 修复 `set-env` 与管理选择不一致时漏判；环境读取/清理失败不删除安装，有隔离回归 |
| 归档链接边界 | 修复规范化目录后相对链接逃逸；Windows 链接测试实际执行，无跳过 |
| 多终端环境更新 | Windows 同用户跨进程互斥，测试包含不同 `JVM_HOME`、超时、错误释放及只读绕过 |
| 构建与脚本 | 重建并同步本地 Windows 两份 exe；Linux amd64 / macOS ARM64 交叉构建通过；Unix 安装与发布脚本 Bash 语法检查通过 |
| 发布版本号 | 手动分支构建保留开发版本，标签构建校验版本格式后注入 |
| 提交范围 | 三份个人学习文档、本地 AI 配置、构建产物与缓存忽略；正式源码、测试和用户文档保留 |

正式稳定版发布前仍需远端 CI 在 Windows/macOS/Linux 和最低支持 Go 版本上实际执行、其他平台运行验收，以及实际 Android Studio/Gradle 项目验证。外部下载源的长期可用性、其他软件的环境修改和操作系统崩溃恢复不属于本工具能保证的范围。
