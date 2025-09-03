# Contributing to JVM Tool

Thank you for your interest in contributing to JVM Tool! This document provides guidelines and information for contributors.

## 🚀 Getting Started

### Prerequisites

- Go 1.21 or higher
- Git
- Basic understanding of Java version management

### Development Setup

1. **Fork and Clone**
   ```bash
   git clone https://github.com/yumu775/jvm.git
   cd jvm
   ```

2. **Install Dependencies**
   ```bash
   go mod tidy
   ```

3. **Build and Test**
   ```bash
   go build -o jvm
   ./jvm --help
   ```

4. **Set up Development Environment**
   ```bash
   # Configure for development
   ./jvm setup
   ```

## 🛠️ Development Guidelines

### Code Style

- Follow standard Go conventions
- Use `gofmt` for formatting
- Add comments for exported functions
- Keep functions focused and small
- Use meaningful variable and function names

### Project Structure

```
jvm-tool/
├── cmd/                 # Command implementations
├── internal/           # Internal packages
│   ├── alias/         # Version alias management
│   ├── config/        # Configuration management
│   ├── download/      # Download functionality
│   ├── env/           # Environment management
│   ├── install/       # Installation logic
│   ├── scanner/       # System scanning
│   ├── sources/       # Download sources
│   ├── uninstall/     # Uninstallation logic
│   └── version/       # Version management
├── main.go            # Entry point
└── README.md          # Documentation
```

### Adding New Commands

1. Create a new file in `cmd/` directory
2. Follow the existing command pattern
3. Add the command to `cmd/root.go`
4. Update help text and documentation
5. Add tests if applicable

### Adding New Features

1. **Plan First**: Open an issue to discuss the feature
2. **Design**: Consider cross-platform compatibility
3. **Implement**: Follow existing patterns
4. **Test**: Test on multiple platforms if possible
5. **Document**: Update README and help text

## 🧪 Testing

### Manual Testing

```bash
# Test basic functionality
jvm list
jvm install latest
jvm use 17
jvm current

# Test platform-specific features
jvm platform info
jvm setup

# Test scanning and importing
jvm scan
jvm import --all
```

### Cross-Platform Testing

- Test on Windows (PowerShell and CMD)
- Test on macOS (Bash and Zsh)
- Test on Linux (Bash, Zsh, Fish)

## 📝 Commit Guidelines

### Commit Message Format

```
<type>(<scope>): <description>

[optional body]

[optional footer]
```

### Types

- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes
- `refactor`: Code refactoring
- `test`: Adding tests
- `chore`: Maintenance tasks

### Examples

```
feat(alias): add support for custom version aliases

fix(windows): resolve symbolic link detection issue

docs(readme): update installation instructions

refactor(config): simplify configuration loading logic
```

## 🐛 Bug Reports

When reporting bugs, please include:

1. **Environment Information**
   - Operating system and version
   - Go version
   - JVM tool version

2. **Steps to Reproduce**
   - Clear, step-by-step instructions
   - Expected vs actual behavior

3. **Additional Context**
   - Error messages
   - Log output
   - Screenshots if applicable

## 💡 Feature Requests

For feature requests:

1. **Check Existing Issues**: Avoid duplicates
2. **Describe the Problem**: What need does this address?
3. **Propose a Solution**: How should it work?
4. **Consider Alternatives**: Are there other approaches?

## 🔄 Pull Request Process

1. **Create a Branch**
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **Make Changes**
   - Follow coding guidelines
   - Add tests if applicable
   - Update documentation

3. **Test Thoroughly**
   - Test on your platform
   - Consider cross-platform implications

4. **Submit PR**
   - Clear title and description
   - Reference related issues
   - Include testing notes

5. **Address Feedback**
   - Respond to review comments
   - Make requested changes
   - Keep the PR updated

## 📚 Documentation

### Updating Documentation

- Update README.md for user-facing changes
- Update help text for command changes
- Add examples for new features
- Update CHANGELOG.md

### Writing Style

- Use clear, concise language
- Include practical examples
- Consider different user skill levels
- Test all documented commands

## 🤝 Community

### Code of Conduct

- Be respectful and inclusive
- Focus on constructive feedback
- Help others learn and grow
- Maintain a welcoming environment

### Getting Help

- Check existing documentation
- Search existing issues
- Ask questions in discussions
- Be specific about your problem

## 🎯 Areas for Contribution

### High Priority

- Cross-platform testing and fixes
- Performance improvements
- Error handling enhancements
- Documentation improvements

### Medium Priority

- New download sources
- Additional shell support
- GUI development
- IDE integrations

### Low Priority

- Code refactoring
- Additional aliases
- Cosmetic improvements
- Advanced features

## 📄 License

By contributing to JVM Tool, you agree that your contributions will be licensed under the MIT License.

---

Thank you for contributing to JVM Tool! Your efforts help make Java version management easier for everyone. 🙏
