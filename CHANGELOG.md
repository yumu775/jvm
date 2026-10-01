# 更新记录

本文件记录实际行为变化。尚未正式发布的变更集中在 Unreleased。

## [Unreleased] — 1.1.0-dev

### Windows 构建与一键配置

- 新增 `build.ps1`，构建成功后同步更新 `build/jvm.exe` 和根目录工具；支持 `-Setup` 连续配置。
- 双击 `jvm.exe` 改为中文配置引导，确认后配置用户 PATH，取消正常退出。
- 新增 `install-jvm.bat` 双击安装入口；PowerShell 安装脚本默认更新旧工具、配置 profile 并激活当前会话。
- 新增 `setup --powershell-profile`，保留其他配置；修复 UTF-8 BOM 标记识别并拒绝破损编码。
- 修复 Windows PowerShell 5 在构建替换时把空备份路径误绑定为空字符串的问题。

### 命令体验与多源恢复

- 新增 `list available` / `available` 和 `ls` / `ls-remote` 兼容入口，可安装列表按主版本、发行版展示最新补丁，支持历史、LTS、来源、JSON 和详情筛选。
- 恢复 Zulu、Corretto、GraalVM Community 的真实自动下载支持；Temurin 增加同厂商 Foojay 元数据备用路径。
- 目录独立缓存、离线读取、陈旧缓存标记、并行查询及部分源失败保留结果。
- 新增 `sources check/default/priority`，默认安装厂商可配置且不会静默更换。
- 安装身份包含发行版，保存真实版本与厂商；多个厂商匹配简写时要求明确 ID。
- alias 命令共享目录缓存和默认来源，支持 `--source/--refresh/--offline`。
- 接通 `config default-version` → `use default` 与本地列表 `auto-scan`；下载源配置展示与真实状态一致。

### 修复

- 卸载同时核对配置选择、持久 JAVA_HOME 和当前进程实际 Java；修复 `set-env` 指向其他已管理版本时漏判活动版本的问题，环境检查或清理失败时保留安装。
- Windows 环境变更命令使用同一用户的跨进程互斥锁，覆盖环境与配置更新，防止不同终端交错写入；超时明确失败，临时激活和预览不加锁。
- 安装目录规范化后重新验证符号链接边界，拒绝因目录上移而越出安装根的链接。
- 手动分支构建保留开发版本号，标签构建校验版本格式后注入，避免将分支名写成工具版本。

- Windows Java 持久配置统一写入用户环境，保留现有 PATH 类型和无关条目；写入失败尝试恢复。
- 不再把子进程环境设置报告为当前 shell 已激活，CMD 不再仅输出手动说明后冒充配置成功。
- `use --temp` 输出指定 shell 的激活脚本，不修改全局选择。
- `current` 与环境诊断区分保存的选择、持久环境和实际执行路径。
- 安装和下载目录配置生效；自定义安装与外部导入登记真实位置。
- Windows 导入不再依赖符号链接。取消外部登记保留原文件。
- 完整版本 ID、数字前缀与数字排序；修复 LTS 偏移未按主版本去重。
- 强制安装改为暂存验证、备份替换、失败回滚。
- 下载验证 SHA-256 和可靠的精确尺寸，拒绝残包及不安全归档路径；Azul API 约整大小仅作参考，避免完整下载后误报失败。
- 修复扫描根目录漏检，限制扫描深度与 Java 执行时间。
- 卸载检查活动版本、所有权和目录边界，清理选择记录，停止按文件名子串误删缓存。
- 安装脚本统一调用 setup，不再包含易覆盖用户 PATH 的 BAT 注册表兜底。
- 核心错误返回非零，不提前报告成功。

### 新增

- `JVM_HOME` 绝对根目录覆盖。
- `config set install-dir/download-dir`；修改仓库前登记旧安装。
- `env --shell <shell>` 纯激活脚本与 `init powershell` 自动激活集成。
- Adoptium 真实官方元数据、分页查询、完整版本与校验和。
- 配置短锁、原子写入；HTTP、文件事务、环境替身和跨模块回归测试。
- Windows/macOS/Linux CI、构建产物及校验清单工作流。
- 使用、迁移、IDE 排查、架构与开发文档。

### 兼容性与限制

- `env --shell` 从布尔参数改为字符串参数；旧诊断功能改为 `--show-shell`。
- 自动下载支持 Temurin、Zulu、Corretto、GraalVM Community；Oracle 仍需手动导入，不再返回占位数据。
- `setup --system-wide` 明确未支持；不覆盖机器级 PATH 或 IDE JDK 设置。
- 外部导入与旧缓存采用保守删除策略。
- 新程序不会自动替换历史二进制或迁移整个数据仓库，见 [迁移说明](docs/migration.md)。

## [1.0.0] — 历史基线

提供 Go/Cobra CLI、版本目录管理、扫描与导入入口、项目配置、环境脚本和下载源原型。此前文档将部分原型能力描述为完整功能；本次审查确认环境切换、目录配置与多源下载存在未接通或占位实现，以上 Unreleased 记录以实际修复结果为准。
