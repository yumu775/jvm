# 环境与 IDE 排查

## 先确认运行的是哪个 JVM 工具

PowerShell：

```powershell
Get-Command jvm -All
jvm --version
jvm config list
jvm env
```

CMD：

```bat
where jvm
jvm --version
jvm config list
jvm env
```

旧的 `jvm.exe`、PowerShell 同名函数、不同的 `JVM_HOME` 都可能使两个终端实际运行不同程序或读取不同配置。PowerShell 集成函数绑定生成它的可执行文件绝对路径；更换工具目录后需重新加载。

## `jvm current` 与 `java -version` 不同

`current` 是保存的管理版本选择，`java -version` 使用当前进程 PATH。`set-env` 还可以独立选择未登记的外部 Java。用 `jvm env` 分别检查管理选择、Windows 用户持久环境、当前 JAVA_HOME 和实际 Java 路径。

在当前 PowerShell 激活：

```powershell
jvm env --shell powershell | Out-String | Invoke-Expression
Get-Command java -All
$env:JAVA_HOME
java -version
javac -version
```

在 CMD 激活：

```bat
jvm env --shell cmd > "%TEMP%\jvm-activate.cmd" && call "%TEMP%\jvm-activate.cmd"
where java
echo %JAVA_HOME%
java -version
```

子进程无法修改父终端。新建 Windows Terminal 标签页也可能继承仍在运行的 Terminal 进程的旧环境；从已刷新环境启动新进程，或在标签页显式激活。通知失败时命令会明确说明环境已保存，可能需重新登录。

Windows 机器 PATH 通常排在用户 PATH 前面。Oracle `javapath` 等机器级入口可能优先于用户 Java。JVM 不擅自删除这些入口；当前 shell 激活会把所选 JDK 放到当前 PATH 开头。需要全局排除冲突时，由用户检查系统环境设置。

## Android Studio / Gradle

1. 使用 `jvm list` 找到目标 JDK 的完整路径，确认该目录有 `bin/java.exe` 和 `bin/javac.exe`（Unix 无 `.exe`）。
2. 在 Android Studio 设置中找到 **Build Tools → Gradle → Gradle JDK**，选择目标 JDK，或确认所选 JAVA_HOME/宏实际解析到该路径。不同 Studio 版本的菜单名称可能略有差异。
3. 检查项目或用户 `gradle.properties` 的 `org.gradle.java.home`；检查 IDE 的 Gradle JDK、本地 Gradle Java Home 配置以及 Gradle Daemon JVM criteria 是否覆盖了环境变量。
4. 在项目目录执行 `gradlew.bat --version`（Unix 为 `./gradlew --version`），核对 Gradle 使用的 JVM，而不是只看 `java -version`。
5. 如需重建旧 Daemon，先停止构建，再执行 `gradlew.bat --stop`，重新同步项目。IDE 必要时重启。

Android Studio 的内置 JBR 负责运行 IDE，本身不必与 Gradle JDK 相同。选择 JDK 还必须满足项目 Gradle 和 Android Gradle Plugin 的兼容要求。JVM 不自动改动这些项目设置。

## 自定义目录不可见

```text
jvm config get install-dir
jvm config get download-dir
jvm list
jvm import "D:\ExistingJava\jdk-17"
```

新版本会登记 `install --dir` 的真实位置。旧版本曾安装到自定义路径但未登记的 JDK，需要显式导入。`setup --path` 只改变工具位置，`config add-path` 只增加扫描位置，二者都不是 JDK 仓库配置。

## 下载和安装失败

- 先运行 `jvm sources list`、`jvm sources check`。Temurin、Zulu、Corretto、GraalVM Community 支持自动下载；Oracle 采用手动导入。
- `jvm list available` 显示紧凑可安装列表；`--refresh` 刷新，`--offline` 仅看已有目录。部分源失败时仍展示其他源，`stale` 表示陈旧缓存而非实时可用性。
- 默认厂商不可用时，可显式执行 `jvm install 17 --source zulu` 或 `jvm sources default zulu`。不会未经说明换掉 JDK 厂商。Temurin 的同厂商备用元数据不意味着原下载服务器也有镜像。
- 版本列表需要网络；HTTP 错误会作为失败返回，不使用硬编码版本冒充真实列表。
- `sources check` 仅检查元数据；真正安装还需取得包校验和并访问文件服务器。元数据服务可用不代表安装包下载必然成功。
- 校验失败会拒绝安装。已有缓存会重新验证，不会直接信任残包。
- 安装在目标仓库的临时目录完成，因此需要足够空间同时容纳归档、新 JDK 和重装备份。
- Windows 无法替换正在被进程占用的 JDK 时，先关闭相关构建或 IDE 再重试；不要手动删除正在使用的目录。
- 正常失败会清理临时文件并回滚。进程被强制终止或断电可能遗留 `.install-lock`、`.jvm-stage-*`、`.jvm-backup-*`。确认没有安装进程后，先核对备份和登记记录，再处理遗留文件；工具不会自动抢占锁或猜测删除备份。

## PowerShell profile 迁移

旧版可能在 Windows PowerShell 5 和 PowerShell 7 的 profile 中写死 Java 路径。新版本会清理用户 Documents 下常见路径中的旧 JVM 标记块，但不会猜测修改 OneDrive 重定向或其他自定义 profile。

若仍被旧值覆盖，检查实际 `$PROFILE` 及其加载的文件，移除旧的 JVM Java 配置块。遇到不成对的 START/END 标记，工具拒绝覆盖，需先修复该文件。执行策略由用户管理，JVM 不修改系统执行策略。
