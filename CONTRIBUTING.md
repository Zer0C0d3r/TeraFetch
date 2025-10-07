# Contributing to TeraFetch

Thank you for your interest in contributing to TeraFetch! This guide will help you get started.

## 🚀 Quick Start

1. **Fork** the repository
2. **Clone** your fork: `git clone https://github.com//terafetch.git`
3. **Create** a branch: `git checkout -b feature/your-feature`
4. **Make** your changes
5. **Test** your changes: `make test`
6. **Commit** and **push**: `git commit -m "Add feature" && git push origin feature/your-feature`
7. **Open** a Pull Request

## 🎯 Priority Areas

### High Impact Contributions
- **Web Scraping**: Improve HTML parsing and JavaScript execution
- **API Reverse Engineering**: Discover new Terabox endpoints and methods
- **Authentication Bypass**: Develop more robust bypass techniques
- **Performance**: Optimize memory usage and download speeds

### Medium Impact
- **Testing**: Add test cases for edge cases and error scenarios
- **Documentation**: Improve usage examples and troubleshooting
- **Platform Support**: Add support for additional architectures
- **Code Quality**: Refactoring and performance improvements

## 🛠 Development Setup

### Prerequisites
- Go 1.25+
- Git
- Make (optional)

### Setup
```bash
git clone https://github.com/Zer0C0d3r/terafetch.git
cd terafetch
go mod tidy
make build
make test
```

## 📝 Guidelines

### Code Style
- Follow [Effective Go](https://golang.org/doc/effective_go.html)
- Use `gofmt` for formatting
- Add comments for exported functions
- Keep functions small and focused

### Testing
- Write tests for new functionality
- Ensure all tests pass: `make test`
- Include both positive and negative test cases

### Commits
- Use clear, descriptive commit messages
- Keep commits focused on a single change
- Reference issues when applicable

## 🐛 Reporting Issues

### Bug Reports
Include:
- OS and architecture
- TeraFetch version
- Steps to reproduce
- Expected vs actual behavior
- Error messages or logs

### Feature Requests
Include:
- Clear description of the feature
- Use case and motivation
- Possible implementation approach

## 🔍 Help Wanted

### Expertise Needed
- **Web Scraping Specialists**: Advanced HTML/JS parsing
- **Reverse Engineers**: Terabox API analysis
- **Security Researchers**: Authentication and bypass methods
- **Performance Engineers**: Large file optimization
- **Go Developers**: Core functionality improvements

### Specific Tasks
- Improve web scraping reliability for complex pages
- Discover new Terabox API endpoints
- Enhance authentication bypass methods
- Optimize memory usage for large downloads
- Add support for batch downloads
- Improve error handling and user feedback

## 📚 Resources

- [Go Documentation](https://golang.org/doc/)
- [Cobra CLI Framework](https://github.com/spf13/cobra)
- [Project Architecture](README.md#development)

## 💬 Getting Help

- **Issues**: [GitHub Issues](https://github.com/Zer0C0d3r/terafetch/issues)
- **Discussions**: [GitHub Discussions](https://github.com/Zer0C0d3r/terafetch/discussions)

## 🏆 Recognition

Contributors will be recognized in:
- CHANGELOG.md for significant contributions
- README.md contributors section
- Release notes

---

**Note**: By contributing, you agree that your contributions will be licensed under the MIT License.