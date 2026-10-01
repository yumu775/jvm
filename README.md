# JVM — Java 版本管理工具

JVM 是使用 Go 编写的命令行工具，用于安装 JDK、登记已有 Java、选择默认版本和激活当前终端。支持 Windows、macOS 和 Linux。

> 当前开发版本：1.1.0-dev。升级旧版本请先阅读 [迁移说明](docs/migration.md)。

## 能力与边界

- 查询 Temurin、Zulu、Corretto、GraalVM Community 的真实发行版本，支持主版本、完整版本、`latest`、`lts`、`lts-N`。
- 下载校验 SHA-256，暂存安装、验证 JDK 结构与 Java 主版本后替换；登记失败恢复旧安装。
- 支持其他磁盘、自定义仓库与缓存目录，所有安装位置统一登记。
- 导入外部 Java 不复制文件、不要求创建符号链接；卸载导入项只取消登记。
- Windows 写入当前用户的 `JAVA_HOME` 和 `Path`，保留其他工具路径；PowerShell 可加载函数实现一次 `use` 同步激活。
- 支持项目 `.jvmrc`、`.java-version`、`.sdkmanrc`。
- Temurin 使用 Adoptium API，失败时尝试同厂商 Foojay 元数据；Zulu 使用 Azul 官方 API；Corretto、GraalVM Community 使用 Foojay 聚合元数据并校验发行包。Oracle 仍采用手动安装后导入。
- 不自动覆盖机器级 PATH，不强制改写 IDE 的 Gradle JDK，不修改已经运行的进程。系统级 `setup --system-wide` 暂不支持。

## 安装脚本怎么选

不需要把所有脚本运行一遍，按使用场景选一个入口即可：

| 场景 | 入口 |
|---|---|
| Windows 源码构建并配置 | `.\build.ps1 -Setup`；只构建用 `.\build.ps1` |
| PowerShell 安装或更新已有工具 | `.\quick-setup.ps1`，指定位置加 `-InstallPath "D:\Tools\jvm"` |
| Windows 双击安装 | `install-jvm.bat`，完成后打开新终端 |
| CMD / 批处理配置 | `setup-jvm.bat`，指定位置用 `--path`，覆盖加 `--force` |
| Linux/macOS 已有可执行文件 | `bash "./install-jvm.sh" --install-path "$HOME/.local/bin"` |

详细参数、文件依赖、更新步骤、环境生效范围与卸载配置见 **[构建与安装脚本使用指南](docs/installation.md)**。这些脚本配置 JVM 工具，JDK 另外使用 `jvm install` 安装。

## 构建

开发环境需要 Go 1.21 或更高版本；构建后的可执行文件运行时不需要 Go。

```powershell
.\build.ps1
.\jvm.exe --version
```

Windows 构建脚本在编译成功后更新 `build/jvm.exe` 和仓库根目录 `jvm.exe`，避免安装脚本继续使用遗留旧程序。编译失败不会替换已有程序；根目录程序被占用时会提示关闭后重试。单独构建不修改环境变量。需要构建后立即配置时，执行 `.\build.ps1 -Setup`，它会调用 `quick-setup.ps1`。

如 PowerShell 执行策略阻止本地脚本，可只对本次进程使用 `powershell -NoProfile -ExecutionPolicy Bypass -File .\build.ps1`，无需修改系统执行策略。

Linux/macOS：

```sh
go build -trimpath -o build/jvm .
./build/jvm --version
```

直接使用 `go build -trimpath -o build/jvm.exe .` 只更新指定产物，不会同步根目录程序；Windows 源码开发建议使用 `build.ps1`。发布工作流生成六种平台/架构的压缩包及 SHA-256 清单，产物作为 Actions artifact 保存，不自动发布 GitHub Release。

## Windows 快速开始

以下命令中的目录可按需修改。工具目录、JDK 仓库、缓存目录是三个独立配置。

JVM 是命令行工具，通常在 PowerShell 或 CMD 中运行。旧版双击出现的 “This is a command line tool” 来自 Cobra 的启动拦截。新版双击 `jvm.exe` 会显示中文引导，确认后配置当前目录的工具 PATH；双击发行包中的 **`install-jvm.bat`** 则同时配置 PowerShell 集成，并保留结果窗口。

```powershell
# 安装/更新工具，配置用户 PATH、当前会话和后续 PowerShell 集成
.\quick-setup.ps1 -InstallPath "D:\Tools\jvm"

# 也可以省略 InstallPath，直接使用当前目录的程序
jvm config set install-dir "D:\Java\versions"
jvm config set download-dir "D:\Java\downloads"

jvm install 17
jvm use 17
java -version
jvm env
```

加载集成后，`jvm use`、`jvm set-env`、`jvm project use` 成功时会同步激活当前 PowerShell。`quick-setup.ps1` 默认把集成写入运行它的 PowerShell 的 `$PROFILE.CurrentUserAllHosts`，保留其他配置，并默认更新目标旧程序（`-Force:$false` 可禁止覆盖）。`-NoProfile` 只跳过持久 profile 集成，当前会话仍加载函数。Windows PowerShell 和 PowerShell 7 使用不同 profile，如两个都使用，应分别运行该脚本。

仅运行 `jvm setup` 时不会自动写入 profile。也可以手动加载：

```powershell
& "D:\Tools\jvm\jvm.exe" init powershell | Out-String | Invoke-Expression
```

直接运行 `jvm.exe use 17` 只保存持久环境。未加载集成时，随后执行：

```powershell
jvm env --shell powershell | Out-String | Invoke-Expression
```

### CMD

```bat
jvm use 17
jvm env --shell cmd > "%TEMP%\jvm-activate.cmd" && call "%TEMP%\jvm-activate.cmd"
java -version
```

输出的 CMD 脚本按批处理文件使用。包含 CMD 无法安全表达的特殊字符时会明确报错，建议使用 PowerShell。

Windows 安装脚本 `setup-jvm.bat`、`quick-setup.ps1` 与 `jvm setup` 使用同一套配置逻辑。源码开发先运行 `build.ps1`，再运行安装脚本；也可用 `build.ps1 -Setup` 连续完成构建和配置。

## Linux / macOS

```sh
./build/jvm setup --path "$HOME/.local/bin"
# 当前终端按需添加工具 PATH；新终端读取 shell 配置
export PATH="$HOME/.local/bin:$PATH"
jvm install 17
jvm list
jvm use 17
eval "$(jvm env --shell bash)"      # Zsh 将 bash 改成 zsh
java -version
```

Fish 用 `jvm env --shell fish | source`。Unix 持久环境更新当前登录 shell 的配置文件，桌面程序不保证读取该文件。

## 查看可安装版本

```text
jvm list available                  # 每个主版本、每个发行版的最新补丁
jvm list available --lts            # 只看长期支持版本
jvm list available 17               # Java 17 的各发行版
jvm list available 17 --all         # 查看 Java 17 历史补丁
jvm list available --source zulu    # 只看 Zulu
jvm list available --refresh        # 强制刷新远端目录
jvm list available --offline        # 只读本地缓存
jvm list available --json           # 结构化结果，包含逐源状态
jvm list available 17 --details     # 包名、下载地址、校验信息
```

默认表格为 `VERSION / LTS / VENDOR / INSTALLED`，不堆积长下载链接。`INSTALLED=yes` 表示完整版本与发行版都匹配，旧记录无法确认厂商时显示 `version-only`。`jvm available`、`jvm list-remote`、`jvm ls-remote` 等价；`jvm ls` 等价于本地列表。

目录缓存有效期 6 小时。源失败时保留其他源的结果，并可使用不超过 30 天的旧缓存，明确显示 `stale`。缓存按平台、发行版、主版本和是否完整历史分开保存；离线查询需要先在线缓存同一范围。缓存不保证对应下载链接永久有效。

## 安装、导入与切换

```text
jvm list available
jvm install 17
jvm install lts
jvm install 17 --source zulu
jvm install 17 --dir "D:\OtherJava"
jvm install 17 --force
jvm list
jvm import "D:\ExistingJava\jdk-17"
jvm import --all --from "D:\ExistingJava"
jvm use 17
jvm current
jvm env
```

新安装按“真实完整版本号-发行版”登记，如 `17.0.12+7-adoptium`，不同厂商可以并存。数字前缀如 `17` 在只有一个匹配厂商时选择其最高已安装补丁；匹配多个厂商时要求使用 `jvm list` 中的完整 ID，避免静默换厂商。精确旧目录名优先，兼容已有 `java-17` 目录。

`--dir` 只影响本次安装，新位置会登记；它不会迁移已存在的同版本。修改默认仓库时会登记原仓库内可识别的版本，保留原文件位置。

临时切换不修改默认版本或持久环境：

```powershell
jvm use 17 --temp --shell powershell | Out-String | Invoke-Expression
```

直接选择外部 Java：

```powershell
jvm set-env "D:\ExistingJava\jdk-17"
jvm env --shell powershell | Out-String | Invoke-Expression
```

自动安装要求 JDK；导入及直接设置可接受 Java 运行环境，但 Android/Gradle 开发应选择包含 `javac` 的完整 JDK。

## 项目版本

```text
jvm project init 17
jvm project get
jvm project use
jvm project set 21
```

从当前目录向父目录查找配置。`project use` 显式选择并持久化项目版本；不会在 `cd` 时自动运行，也不会覆盖 Android Studio 的 Gradle JDK 设置。并行开发不同版本的项目时，使用临时激活，避免反复改变全局默认选择。

## 配置与目录

```text
jvm config list
jvm config set install-dir "D:\Java\versions"
jvm config set download-dir "D:\Java\downloads"
jvm config add-path "D:\ExistingJava"
jvm sources list
jvm sources check                   # 检查启用源的 Java 17 元数据
jvm sources check zulu --major 21
jvm sources default zulu            # 默认安装选择 Zulu
jvm sources priority zulu 0          # 目录/来源显示与查询顺序，数字小优先
jvm sources disable adoptium
jvm sources enable adoptium
```

默认安装来源初始为 `adoptium`。`--source` 优先于默认来源；失败时不会自动改装另一厂商。可显式选择其他来源，或修改默认值。来源优先级只影响顺序，不会改写默认安装厂商。旧配置中已禁用的来源继续保持禁用，需按需 `sources enable zulu/corretto/graalvm`。

`jvm alias resolve lts --source zulu` 与 `jvm install lts --source zulu` 使用相同来源策略。全部 alias 子命令支持 `--source`、`--refresh`、`--offline`；无离线缓存时仍可解释别名定义，但不能声称已经解析出最新版本。

```text
jvm config set default-version <完整已安装ID>
jvm use default
jvm config set auto-scan true
```

`default-version` 是显式 `use default` 使用的本地版本别名，与默认下载厂商不同。`auto-scan` 控制没有托管版本时 `jvm list` 是否自动发现系统 Java；已有托管版本时不会额外扫描，显式 `--system/--all` 始终扫描。

默认根目录是用户目录下的 `.jvm`，可在启动进程前通过 **绝对路径** `JVM_HOME` 覆盖：

```powershell
$env:JVM_HOME = "D:\JVMData"
```

这只影响当前进程及其子进程；其他 CMD/PowerShell 需使用相同设置。改变 `JVM_HOME` 等于选择另一套配置，不会自动迁移旧数据。仅需把 JDK/缓存移出 C 盘时，优先使用 `config set install-dir/download-dir`。

```text
<JVM_HOME 或 ~/.jvm>/
  config.json       # 安装记录、目录与保存的版本选择
  sources.json      # 下载源启停
  versions/         # 默认 JDK 仓库，可改到其他位置
  downloads/        # 默认下载缓存，可改到其他位置
```

旧 `download_sources` 字段保留读取兼容，实际来源以 `jvm sources` 为准，配置列表展示真实来源状态。

## 卸载

```text
jvm uninstall 17 --dry-run
jvm uninstall 17
jvm uninstall 17 --force
```

- 工具安装的版本：删除已登记的 JDK 目录。
- 外部导入的版本：取消登记，保留外部文件。
- 当前默认版本受保护，`--force` 明确允许卸载。
- 卸载当前选择时尝试清理匹配的持久 Java 环境；已有终端仍需重新选择并激活。
- 缓存不再通过文件名子串猜测归属并删除；不能确认归属的缓存保留。
- `jvm setup --uninstall` 只移除工具 PATH 配置，不删除 JDK 数据。

## 排查与开发

- [环境变量、Android Studio 与 Gradle 排查](docs/troubleshooting.md)
- [旧版本迁移与兼容变化](docs/migration.md)
- [架构与数据一致性](docs/architecture.md)
- [开发、测试与发布规范](CONTRIBUTING.md)
- [更新记录](CHANGELOG.md)
- [本次验证记录与实机验收范围](docs/verification.md)
- [完整命令对照与功能边界](docs/commands.md)

```sh
go test ./...
go vet ./...
```

测试使用临时目录、模拟环境存储和本地 HTTP 服务，不安装真实 JDK、不写真实用户环境。跨平台 CI 会执行格式检查、静态检查和回归测试。

## 许可证

[MIT](LICENSE)
