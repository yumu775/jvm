# 从 1.0 迁移

## 升级前

记录 `where jvm` 或 `Get-Command jvm -All` 的结果，备份旧的 `.jvm/config.json`、相关 PowerShell/Unix profile。Windows 推荐运行 `build.ps1`，构建成功后同步更新 `build/jvm.exe` 与根目录程序，再使用 `quick-setup.ps1` 配置；也可用 `build.ps1 -Setup` 连续完成。独立工具目录需要另行指定 `-InstallPath` 更新该副本。直接执行 `go build -o build/jvm.exe .` 或 Unix 的 `go build -o build/jvm .` 只更新指定产物，不自动配置环境。完整步骤见[安装指南](installation.md)。

## 数据兼容

默认继续读取用户目录下 `.jvm/config.json`。旧 `java-17` 或版本号目录仍可发现；精确旧 ID 优先于主版本前缀解析。

新增 `installations` 映射，记录版本 ID、绝对目录和 `managed` 所有权：

```json
{
  "install_dir": "D:\\Java\\versions",
  "download_dir": "D:\\Java\\downloads",
  "current_version": "17.0.12+7",
  "installations": {
    "17.0.12+7": {
      "path": "D:\\Java\\versions\\java-17.0.12+7",
      "managed": true
    }
  }
}
```

这里只是结构示例，不应直接覆盖现有配置。托管记录由安装流程创建；手动导入记录为 `managed: false`。不要手工把外部目录标记为托管，否则会授予卸载删除权。

设置 `install-dir` 前，工具先登记原仓库中可识别的版本，所以改变仓库只影响将来的安装，不移动旧文件。需要将旧 JDK 物理搬到其他磁盘时，先保存记录并停止相关进程；本版本没有自动搬迁整个仓库的命令。可在新盘重新安装，或对已在新盘的 JDK 执行 `import`。

`JVM_HOME` 是根目录覆盖项，必须使用绝对路径。只给 PowerShell 设置而不给 CMD 设置，会读取不同配置；修改它不会自动复制旧配置。

## 命令行为变化

| 旧行为 | 新行为 |
| --- | --- |
| `use` 提示当前会话已生效，实际只修改子进程 | 保存持久环境，提示激活；PowerShell 可通过 `init powershell` 集成一次切换 |
| `use --temp` 仍修改默认版本 | 需要 `--shell`，只输出脚本，不保存选择 |
| `env --shell` 是布尔诊断开关 | `env --shell powershell/cmd/bash/zsh/sh/fish` 输出纯脚本；诊断改为 `--show-shell` |
| `current` 容易被理解成实际运行版本 | 明确表示保存的选择，实际运行环境由 `env` 诊断 |
| `install latest/lts` 使用别名命名目录 | 使用 API 返回的真实完整版本作为 ID |
| `install --dir` 安装后无法切换 | 登记真实目录，可 list/use/uninstall |
| Windows 导入依赖符号链接或失败 | 登记外部目录，不复制、不要求管理员权限 |
| 卸载可能误删外部文件或缓存 | 外部目录只取消登记，缓存不按版本子串删除 |
| 多下载源显示硬编码旧版本 | Temurin、Zulu、Corretto、GraalVM Community 接入真实元数据与下载校验，Oracle 手动导入 |
| 失败经常仍返回 0 | 核心操作失败返回非零，不提前报告成功 |

旧配置中的 `download_sources` 保留用于兼容读取；下载源由 `sources.json` 和 `jvm sources` 管理。`default_version` 接通 `jvm use default`，新增设置会校验已安装版本。`--persistent` 保留为兼容参数；普通 `use` 已是持久模式。

`sources.json` 兼容旧的来源布尔映射，新格式保存 `enabled`、`priorities`、`default_source`。旧版禁用过的源保持禁用，可用 `jvm sources enable zulu` 等逐一启用。默认下载厂商和本地 `default-version` 是不同设置。

新的安装 ID 增加发行版后缀，记录 `version/source/vendor`，旧 ID 原样保留。旧记录没有厂商信息时不会假设其就是 Temurin；列表只标记 `version-only`。多厂商匹配同一数字简写时使用完整 ID 切换。

## 环境迁移

Windows 新版将 Java 用户环境统一写到 HKCU Environment，不依赖 CMD/PowerShell 类型推断。不自动覆盖机器级 PATH，也不会刷新所有已运行的终端或 IDE。

新版本尝试移除常见 PowerShell profile 中完整的旧 JVM 标记块。重定向、自定义或损坏的 profile 需按[排查手册](troubleshooting.md)检查。无需为了使用 JVM 设置全局脚本执行策略。

升级后推荐依次验证：`jvm list`、`jvm use <版本>`、激活当前 shell、`jvm env`、`java -version`、`javac -version`，最后核对 IDE 和 Gradle 自己选择的 JDK。
