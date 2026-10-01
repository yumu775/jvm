# JVM 命令对照与使用边界

这份清单描述仓库当前提供的命令。命令存在、能保存配置、能改变当前终端，是三件不同的事；下表分别说明实际行为。所有命令均支持 `--help`，完整参数以当前构建的帮助输出为准。

## 常用流程

```powershell
# 持久设置安装位置与下载缓存，可使用其他磁盘
jvm config set install-dir "D:\JavaVersions"
jvm config set download-dir "D:\JavaCache"

# 列出来源配置、检查联网情况、明确默认发行版
jvm sources
jvm sources check
jvm sources default adoptium

# 安装后选择版本
jvm install 17
jvm use 17

# 为当前 PowerShell 加载集成函数，之后普通 jvm use 可自动应用
jvm init powershell | Out-String | Invoke-Expression
jvm use 17
jvm current
```

`jvm init powershell` 本身只输出函数定义，不写入 PowerShell profile。需要每次启动自动加载时，将上面的加载语句放入自己实际使用的 `$PROFILE`；不要把函数定义输出误当成已经配置成功。

已有 CMD 可以显式执行激活脚本：

```bat
jvm env --shell cmd > "%TEMP%\jvm-activate.cmd" && call "%TEMP%\jvm-activate.cmd"
```

已有 Bash / Zsh 可以执行：

```sh
eval "$(jvm env --shell bash)"
```

Windows 普通 `use` 保存当前用户的环境变量。已经打开的终端、IDE 及其父进程仍保留旧环境；PowerShell 集成函数或上述激活命令负责当前终端。Windows 系统 PATH 中的其他 Java 入口可能优先于用户 PATH，`jvm env` 会报告实际解析结果。Android Studio 的 Gradle JDK、IDE 自带运行时是独立设置，必要时显式选择目标 **JDK**。`set-env` 可以使用 JRE，但 JRE 不能代替 Android 构建所需的 JDK。

## 安装、列举与切换

| 命令 | 当前行为 | 主要选项 / 说明 |
|---|---|---|
| `jvm` / `jvm --help` | 显示帮助 | 不执行安装或切换 |
| `jvm --version` | 显示工具版本 | 与 `java -version` 不同 |
| `jvm completion <bash\|fish\|powershell\|zsh>` | Cobra 提供的命令补全脚本生成器 | 输出脚本后需按对应 shell 加载；目前未为已安装版本增加动态补全 |
| `jvm install <version>` | 下载、验证并安装 JDK | `--source` 明确发行版；`--dir` 指定安装仓库；`--force` 替换；`--keep-download` 保留缓存 |
| `jvm list` | 列出已登记 / 已安装版本 | `--system` / `--all` 同时检查系统安装 |
| `jvm list available [version]` | 按主版本 / 发行版显示最新可安装补丁 | 新增；`--source` 可重复或逗号分隔；`--lts`、`--all`、`--json`、`--refresh`、`--offline`、`--details` |
| `jvm list-remote [version]` | 与 `list available` 共享远端目录实现 | 兼容原命令；别名 `ls-remote`、`available`；参数同上 |
| `jvm current` | 同时显示配置选择与当前终端 Java | 不把配置文件的版本当成已经生效的运行时 |
| `jvm use <version>` | 保存默认选择及持久 Java 环境 | 裸可执行文件无法修改父终端；已加载的 PowerShell 集成函数会自动激活 |
| `jvm use <version> --temp --shell <shell>` | 输出临时激活脚本 | 不保存默认选择；需要当前 shell 执行输出 |
| `jvm init powershell` | 输出 PowerShell `jvm` 函数 | 新增；加载后普通 `use`、`set-env`、`project use` 成功时自动激活 |

`--temp` 和 `--persistent` 互斥。`use --shell` 必须同时指定 `--temp`，避免传了参数却被忽略。临时脚本支持 `powershell`、`cmd`、`bash`、`zsh`、`sh`、`fish`。CMD 对含 `%`、`!` 或引号等无法安全表达的路径明确报错，可以使用 PowerShell。

`list available` 默认显示摘要，`list available 17 --all` 显示 Java 17 的补丁历史；指定完整版本时按该版本过滤。`--json` 输出结构化结果，`--details` 附加下载文件及校验信息。`--refresh` 强制刷新，`--offline` 只读本地元数据缓存，两者互斥。远端来源彼此独立，某一来源不可用时仍可显示其他来源结果。注意本地 `list --all` 是“管理版本加系统扫描”，远端 `list available --all` 才是“所有补丁历史”。

## 下载源管理（本轮补齐）

| 命令 | 当前行为 | 网络 / 配置影响 |
|---|---|---|
| `jvm sources` / `jvm sources list` | 紧凑显示默认、启用、优先级、自动下载能力与发行版 | 只读本地配置，不联网 |
| `jvm sources enable <name>` | 持久启用来源 | 不支持自动下载的来源会报错，不是假启用 |
| `jvm sources disable <name>` | 持久禁用来源 | 禁用默认来源后需显式选择其他默认来源 |
| `jvm sources default` | 输出默认安装来源名称 | 只读，可用于脚本 |
| `jvm sources default <name>` | 持久设置默认安装发行版 | 不隐式启用被禁用的来源；未知 / 不支持的来源报错 |
| `jvm sources priority <name> <number>` | 持久设置非负整数优先级 | 越小越靠前；**不改变默认安装发行版** |
| `jvm sources check [name]` | 逐源刷新当前平台 Java 17 元数据并显示结果 | 默认检查已启用来源；可用 `--major 21` 检查其他主版本 |

`sources check` 检查的是版本目录，不会下载完整 JDK，不能保证安装包下载地址此刻也可用。报告区分 `live`（实时）、`cache`（缓存）、`stale`（陈旧缓存）、`error`（失败）、`disabled`（禁用）。某一来源失败时仍显示其他来源结果；出现失败或只能使用陈旧缓存时返回非零退出码。显式检查被禁用的来源时，应先执行 `sources enable`。

默认来源或 `install --source` 选择失败时，安装必须报错，不能自动改装其他 Java 厂商。同一厂商的元数据备用接口与换厂商是不同的行为。Adoptium、Zulu、Corretto、GraalVM 的自动下载能力以来源表显示为准；Oracle 暂需从官方获取后 `jvm import`。不存在“开启一个配置开关，就让尚未实现的供应商接口自动可用”的机制。

## 环境诊断与外部 Java

| 命令 | 当前行为 | 说明 |
|---|---|---|
| `jvm env` | 显示持久用户 Java、继承的 `JAVA_HOME`、配置默认与 PATH 实际解析 | 找不到 / 无法运行 Java 时返回错误 |
| `jvm env --show-shell` | 在诊断中附加 shell 配置信息 | 替代旧的布尔 `--shell` 用法 |
| `jvm env --shell <shell>` | 输出纯激活脚本 | 不夹带成功提示，适合管道执行 |
| `jvm env --shell <shell> --java-version <version>` | 为指定已安装版本生成脚本 | 不保存默认选择 |
| `jvm env --shell <shell> --java-home <path>` | 为指定外部 Java 生成脚本 | 与 `--java-version` 互斥 |
| `jvm set-env <path>` | 直接持久设置外部 Java 环境 | 不把外部目录复制到安装仓库，不改已管理默认版本记录 |
| `jvm set-env <path> --temp --shell <shell>` | 输出外部 Java 的临时激活脚本 | 不写持久环境 |
| `jvm set-env --auto-detect <directory>` | 扫描目录并选择 Java | 多个结果时交互选择；不可与 `--temp` 合用 |
| `jvm set-env list [directory]` | 列出可以使用的 Java 安装 | 不设置环境 |

## 扫描、导入与项目

| 命令 | 当前行为 | 主要选项 / 说明 |
|---|---|---|
| `jvm scan` | 扫描系统和配置的扫描目录 | `--path` 额外目录；`--details` 详细信息；`--import-all` 导入全部结果 |
| `jvm import <version\|path>` | 将现有安装登记到管理列表 | `--from` 限定扫描路径；`--all` 导入找到的全部版本；外部目录不会变成本工具所有 |
| `jvm project` / `jvm project get` | 向上查找并显示项目 Java 配置 | 支持 `.jvmrc`、`.java-version`、`.sdkmanrc` |
| `jvm project init <version>` | 创建 `.jvmrc` | `--force` 覆盖已有配置；`--switch` 随后选择该版本 |
| `jvm project set <version>` | 更新已有项目配置 | `--switch` 随后选择该版本 |
| `jvm project use` | 选择项目声明的 Java 版本 | 当前行为会保存持久选择；不是自动进入目录时切换，也不是隔离的项目子进程 |

## 配置与工具安装

脚本之间的区别、逐项参数与完整操作示例见[构建与安装脚本使用指南](installation.md)。

| 命令 | 当前行为 | 说明 |
|---|---|---|
| `jvm config` / `jvm config list` | 显示配置 | 配置文件所在目录可通过 `JVM_HOME` 指定 |
| `jvm config get <key>` | 读取指定配置 | `current-version`、`default-version`、`auto-scan`、`install-dir`、`download-dir`、`scan-paths` |
| `jvm config set install-dir <path>` | 改变以后安装的默认仓库 | 保留已有版本登记，不自动移动已有 JDK |
| `jvm config set download-dir <path>` | 改变下载缓存目录 | 不自动移动旧缓存 |
| `jvm config set auto-scan <true\|false>` | 控制 `list` 在没有已管理安装时是否补充扫描系统 Java | 不自动导入，也不添加 shell 切换钩子；显式 `list --system` / `scan` 不受该默认开关限制 |
| `jvm config set default-version <value>` | 将已安装版本的明确 ID 保存为 `default` 选择 | 数字简写解析到已安装版本；之后 `jvm use default` 使用该版本；保存配置本身不激活；拒绝不存在的版本和 `default` 自引用 |
| `jvm config add-path <path>` | 添加自定义扫描目录 | 用于发现外部安装 |
| `jvm config remove-path <path>` | 移除自定义扫描目录 | 不删除目录中的 Java |
| `jvm setup` | 把工具目录加入持久 PATH | Windows 当前用户环境；Unix 当前 shell 的工具配置块 |
| `jvm setup --path <directory>` | 复制工具并配置其 PATH | 与 Java 安装仓库不是同一个目录设置；`--force` 允许替换已有工具文件 |
| `jvm setup --powershell-profile <file>` | Windows 下持久加载 PowerShell 集成 | 必须为绝对路径；只维护 JVM 标记块，保留其他内容；与 `--uninstall` 合用移除该集成块 |
| `jvm setup --uninstall [--path <directory>]` | 移除工具的持久 PATH 登记 | 不卸载 Java |
| `jvm setup --system-wide` | 明确报“不支持” | 不再把未实现伪装成缺少管理员权限 |

`config list` 的 `Download Sources` 已改为读取与 `jvm sources` 相同的配置，显示真实启用状态、默认来源与优先级。旧配置文件内 `download_sources` 字段保留兼容读取，但不再作为下载来源的另一套展示入口。

`setup-jvm.bat`、`quick-setup.ps1`、`install-jvm.sh` 均使用脚本所在目录的工具，统一调用 `setup`，保留真实失败结果。PowerShell 脚本支持 `-InstallPath`、`-Force`（默认启用）和 `-NoProfile`；默认配置当前 PowerShell 的用户级 AllHosts profile 并立即加载当前会话集成。Unix 脚本支持 `--install-path`、`--force`。

Windows 源码构建推荐 `build.ps1`，编译成功后同步更新 `build/jvm.exe` 与根目录程序；`build.ps1 -Setup` 随后调用 PowerShell 安装脚本。`install-jvm.bat` 是保留结果窗口的双击安装入口；`setup-jvm.bat` 供命令行调用，不暂停。双击新版 `jvm.exe` 会询问是否配置工具 PATH，不会直接安装或切换 Java。

## 别名与清理

| 命令 | 当前行为 | 说明 |
|---|---|---|
| `jvm alias` / `jvm alias list` | 列出内置版本别名 | 支持 `--source`、`--refresh`、`--offline`；省略来源时遵循 `sources default` |
| `jvm alias resolve <alias>` | 把内置别名解析成版本 | 例如 `latest`、`lts`、`lts-1`、`<source>-latest`；支持同样的来源与缓存选项；需要可用元数据 |
| `jvm alias explain <alias>` | 解释内置别名 | 不创建用户自定义别名 |
| `jvm alias suggest [partial]` | 提供别名 / 版本建议 | 不是 shell 自动补全安装命令 |
| `jvm uninstall <version>` | 卸载已管理安装或移除外部登记 | `--dry-run` 预览；`--force` 允许处理当前版本；外部原目录不应删除 |
| `jvm uninstall list` | 列出可卸载记录 | 只读 |
| `jvm uninstall clean [--dry-run]` | 清理确认路径不存在的安装登记及失效 current/default 引用 | `--dry-run` 只预览；真实操作使用配置事务；权限错误与安装中的目标保留并说明原因 |
| `jvm platform` / `jvm platform info` | 显示平台信息 | 平台信息不代表每个发行版都有该平台安装包 |
| `jvm platform features` | 显示功能说明 | 静态说明不能代替下载源和实际 Java 检查 |
| `jvm platform shells` | 显示 shell 使用信息 | 不安装 shell 集成 |

`alias` 的 `--source`、`--refresh`、`--offline` 由所有子命令继承，刷新与离线互斥；带厂商名称的别名与显式 `--source` 冲突时会报错，不会静默换厂商。

`uninstall clean` 现在是可用的**配置引用清理**，不会删除 Java 安装文件、外部目录、下载缓存或元数据缓存。下载缓存缺少可靠归属时明确保留；现存目录、无法访问的目录和带安装锁的目标不会被当成孤立安装。尚存在的旧安装目录也用于确认 current/default 引用是否有效。此命令不修改系统环境；清理失效选择后，使用有效的 `jvm use <version>` 刷新环境。它与 `uninstall <version>` 的文件卸载行为不同。

`uninstall <version>` 的活动版本检查同时考虑配置选择、Windows 用户持久 `JAVA_HOME`、当前进程 `JAVA_HOME` 和 PATH 实际解析结果。即使通过 `set-env` 选择的版本与配置记录不同，也需要显式 `--force`；无法读取或清理环境时中止卸载，`--dry-run` 不清理环境。

## 尚未实现或不应承诺的能力

| 能力 | 当前边界 / 推荐方式 |
|---|---|
| 对所有已打开的 CMD、PowerShell、IDE 立即切换 | 操作系统不会让子进程重写任意父进程环境；分别激活或重启 |
| 所有 shell 都能裸 `jvm use` 后立即改变当前环境 | 目前 PowerShell 有显式加载的函数集成；其他 shell 执行激活脚本 |
| 自动进入目录后切换项目版本 | 没有自动 `cd` 钩子；使用 `jvm project use` |
| 自动覆盖 Android Studio / Gradle 的独立 JDK 设置 | 不修改 IDE 项目设置；明确选择 Gradle JDK |
| Oracle 自动下载安装 | 尚未实现；官方获取后导入 |
| `alias add` / `alias delete` 用户自定义别名 | 当前是内置别名查询 / 解析接口 |
| `config set download-sources ...` | 不是当前配置命令支持的设置键；使用 `jvm sources` 子命令 |
| 修改安装目录时自动迁移全部已安装 JDK | 当前只改变以后安装位置并保留旧记录 |
| 未经选择自动更换发行版完成安装 | 不提供；默认来源失败需用户显式选择其他来源 |

命令扩展的优先次序建议是：先稳定版本查询、来源选择及错误解释，再统一项目与 shell 工作流，最后补自定义别名、自动补全和自动目录切换。不要仅增加命令名称，而缺少相应的行为、失败语义与测试。
