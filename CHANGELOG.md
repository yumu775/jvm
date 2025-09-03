# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2025-01-XX

### Added
- 🚀 **Core Version Management**
  - Install, uninstall, and switch between Java versions
  - List installed and available versions
  - Current version detection and management

- 📦 **Multi-Source Download Support**
  - Eclipse Adoptium (Temurin)
  - Amazon Corretto
  - Azul Zulu
  - Oracle JDK
  - GraalVM

- 🏷️ **Smart Version Aliases**
  - `latest` - Latest stable version
  - `lts` - Latest LTS version
  - `lts-1`, `lts-2` - Previous LTS versions
  - `stable` - Latest stable version
  - Source-specific aliases (e.g., `adoptium-latest`)

- 🔍 **Intelligent System Scanning**
  - Automatic detection of existing Java installations
  - Custom path scanning support
  - Import existing installations into JVM management

- 🎯 **Project Configuration**
  - `.jvmrc` file support for project-specific Java versions
  - Automatic version switching based on project configuration

- 🗑️ **Safe Uninstallation**
  - Preview mode with `--dry-run`
  - Automatic cleanup of downloads and configuration
  - Protection against uninstalling active versions

- ⚙️ **Configuration Management**
  - Custom scan paths configuration
  - Download source management
  - Auto-scan settings

- 🌍 **Cross-Platform Support**
  - Windows (PowerShell, CMD)
  - macOS (Bash, Zsh)
  - Linux (Bash, Zsh, Fish)

- ⚡ **Environment Setup**
  - One-click environment variable configuration
  - Shell integration scripts
  - Automatic PATH management

- 🔧 **Advanced Features**
  - Direct environment variable setting (`jvm set-env`)
  - Platform information and compatibility checking
  - Flexible import from custom paths
  - Batch operations support

### Technical Highlights
- Built with Go for performance and cross-platform compatibility
- Symbolic link support for efficient storage
- Robust error handling and user feedback
- Colorized output for better user experience
- Comprehensive help system

### Installation Methods
- One-click setup scripts for all platforms
- Built-in `jvm setup` command
- Manual installation support
- Automatic environment variable configuration

## [Unreleased]

### Planned Features
- GUI version for non-technical users
- Integration with popular IDEs
- Automatic Java version detection for projects
- Cloud-based version synchronization
- Plugin system for extensibility
