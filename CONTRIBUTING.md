# Contributing to vpsctl

Thank you for your interest in contributing to vpsctl! This document provides guidelines and instructions for contributing.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [How to Contribute](#how-to-contribute)
- [Pull Request Process](#pull-request-process)
- [Coding Standards](#coding-standards)
- [Commit Messages](#commit-messages)
- [Reporting Bugs](#reporting-bugs)
- [Requesting Features](#requesting-features)
- [Security Issues](#security-issues)

## Code of Conduct

By participating in this project, you agree to:

- Be respectful and inclusive
- Focus on constructive feedback
- Use welcoming and inclusive language
- Show empathy toward other contributors

## Getting Started

1. **Fork** the repository on GitHub
2. **Clone** your fork locally:
   ```bash
   git clone https://github.com/YOUR_USERNAME/vpsctl.git
   cd vpsctl
   ```
3. **Add upstream remote**:
   ```bash
   git remote add upstream https://github.com/vpsctl/vpsctl.git
   ```
4. **Create a branch** for your changes:
   ```bash
   git checkout -b feature/your-feature-name
   ```

## Development Setup

### Prerequisites

- **Go** 1.22 or later
- **Docker** (for testing container management features)
- **golangci-lint** (for linting)
- **air** (optional, for hot reload)

### Setup

```bash
# Install dependencies
go mod download

# Build the project
make build

# Run tests
make test

# Run in development mode
make dev
```

### Useful Commands

| Command | Description |
|---------|-------------|
| `make build` | Build the binary |
| `make run` | Build and run |
| `make dev` | Run with hot reload |
| `make test` | Run all tests |
| `make test-short` | Run tests without integration tests |
| `make lint` | Run linter |
| `make fmt` | Format code |
| `make check` | Run all checks (fmt + vet + lint + test) |
| `make clean` | Clean build artifacts |

## How to Contribute

### Types of Contributions

- **Bug Fixes** — Fix issues in existing functionality
- **Features** — Add new functionality
- **Documentation** — Improve or add documentation
- **Tests** — Add or improve test coverage
- **Refactoring** — Improve code quality without changing functionality
- **Performance** — Optimize performance

### First-Time Contributors

Look for issues labeled with:
- `good first issue` — Simple issues perfect for newcomers
- `help wanted` — Issues where we need community help
- `documentation` — Documentation improvements

## Pull Request Process

### Before Submitting

1. **Ensure your code compiles**:
   ```bash
   make build
   ```

2. **Run all checks**:
   ```bash
   make check
   ```

3. **Update documentation** if your changes affect the API or user-facing features

4. **Add tests** for new functionality

### Submitting a Pull Request

1. **Push your changes**:
   ```bash
   git push origin feature/your-feature-name
   ```

2. **Open a Pull Request** on GitHub

3. **Fill out the PR template** with:
   - Description of changes
   - Motivation for the changes
   - Related issue number (if applicable)
   - Testing performed

4. **Wait for review** — A maintainer will review your PR

5. **Address feedback** — Make requested changes and push again

### PR Guidelines

- Keep PRs focused on a single change
- Write clear commit messages
- Add tests for new functionality
- Update documentation as needed
- Ensure CI passes before requesting review

## Coding Standards

### Go Style

- Follow the [Effective Go](https://go.dev/doc/effective_go) guidelines
- Use `gofmt` and `goimports` for formatting
- Follow the project's existing code style
- Write meaningful variable and function names
- Add comments for complex logic (avoid obvious comments)

### Error Handling

```go
// Good
result, err := doSomething()
if err != nil {
    return fmt.Errorf("failed to do something: %w", err)
}

// Bad
result, _ := doSomething()
```

### Testing

- Write table-driven tests when appropriate
- Use meaningful test names that describe the scenario
- Test both success and error cases
- Use mocks for external dependencies

```go
func TestAuthenticate(t *testing.T) {
    tests := []struct {
        name     string
        username string
        password string
        want     bool
    }{
        {
            name:     "valid credentials",
            username: "admin",
            password: "correct-password",
            want:     true,
        },
        {
            name:     "invalid password",
            username: "admin",
            password: "wrong-password",
            want:     false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := authenticate(tt.username, tt.password)
            if got != tt.want {
                t.Errorf("authenticate() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Project Structure

```
vpsctl/
├── cmd/
│   └── vpsctl/         # CLI entry point
├── internal/
│   ├── api/            # HTTP handlers and routes
│   ├── auth/           # Authentication logic
│   ├── config/         # Configuration management
│   ├── docker/         # Docker integration
│   ├── models/         # Data models
│   ├── store/          # Database layer
│   └── ws/             # WebSocket handlers
├── web/                # Frontend assets
├── scripts/            # Build and install scripts
├── docs/               # Documentation
├── configs/            # Example configuration files
└── Makefile
```

## Commit Messages

### Format

```
<type>(<scope>): <subject>

<body>

<footer>
```

### Types

- **feat**: New feature
- **fix**: Bug fix
- **docs**: Documentation changes
- **style**: Code style changes (formatting, etc.)
- **refactor**: Code refactoring
- **test**: Adding or updating tests
- **chore**: Maintenance tasks
- **perf**: Performance improvements

### Examples

```
feat(docker): add container resource limits API

Added endpoint to configure CPU and memory limits for Docker containers.
This allows users to set resource constraints through the web interface.

Closes #123
```

```
fix(auth): prevent JWT token reuse after logout

Implemented token blacklisting to ensure tokens are invalidated
immediately upon user logout.

Fixes #456
```

## Reporting Bugs

### Before Reporting

1. **Search existing issues** to avoid duplicates
2. **Test with the latest version** to ensure the bug still exists
3. **Collect information** about your environment

### Bug Report Template

```markdown
**Describe the bug**
A clear description of what the bug is.

**To reproduce**
Steps to reproduce the behavior:
1. Go to '...'
2. Click on '...'
3. See error

**Expected behavior**
What you expected to happen.

**Screenshots**
If applicable, add screenshots.

**Environment**
- OS: [e.g., Ubuntu 22.04]
- vpsctl version: [e.g., 1.0.0]
- Browser: [e.g., Chrome 120]
- Docker version: [e.g., 24.0]

**Additional context**
Any other context about the problem.
```

## Requesting Features

### Feature Request Template

```markdown
**Is your feature request related to a problem?**
A clear description of the problem. Ex. "I'm frustrated when..."

**Describe the solution you'd like**
A clear description of what you want to happen.

**Describe alternatives you've considered**
Any alternative solutions or features you've considered.

**Additional context**
Add any other context or screenshots about the feature request.
```

## Security Issues

**DO NOT** open a public issue for security vulnerabilities.

Instead, please follow our [Security Policy](SECURITY.md) to report security issues privately.

## Questions?

If you have questions about contributing:

1. Check the [README](README.md) for general information
2. Open a [Discussion](https://github.com/vpsctl/vpsctl/discussions) for questions
3. Join our community channels (links in README)

Thank you for contributing to vpsctl!
