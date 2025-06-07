# Contributing to Streamer

We're excited that you're interested in contributing to Streamer! This document provides guidelines for contributing to the project.

## Code of Conduct

By participating in this project, you agree to abide by our Code of Conduct:
- Be respectful and inclusive
- Welcome newcomers and help them get started
- Focus on constructive criticism
- Respect differing viewpoints and experiences

## How to Contribute

### Reporting Bugs

1. Check if the bug has already been reported in [Issues](https://github.com/pay-theory/streamer/issues)
2. If not, create a new issue with:
   - Clear, descriptive title
   - Steps to reproduce
   - Expected vs actual behavior
   - Environment details (Go version, OS, etc.)
   - Relevant logs or error messages

### Suggesting Features

1. Check existing [Issues](https://github.com/pay-theory/streamer/issues) for similar suggestions
2. Create a new issue with:
   - Clear use case
   - Proposed solution
   - Alternative solutions considered
   - Potential impact on existing functionality

### Contributing Code

#### Setup

1. Fork the repository
2. Clone your fork:
   ```bash
   git clone https://github.com/your-username/streamer.git
   cd streamer
   ```
3. Add upstream remote:
   ```bash
   git remote add upstream https://github.com/pay-theory/streamer.git
   ```
4. Create a feature branch:
   ```bash
   git checkout -b feature/your-feature-name
   ```

#### Development Process

1. **Write tests first** (TDD approach)
2. **Implement your feature**
3. **Ensure all tests pass**: `make test`
4. **Check code formatting**: `go fmt ./...`
5. **Run linter**: `golangci-lint run`
6. **Update documentation** if needed

#### Commit Guidelines

Follow conventional commits format:

```
type(scope): subject

body

footer
```

Types:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation only
- `style`: Code style changes
- `refactor`: Code refactoring
- `test`: Test additions/changes
- `chore`: Build process or auxiliary tool changes

Example:
```
feat(router): add request prioritization

Implement priority queue for async requests based on user tier
and request type. High-priority requests are processed first.

Closes #123
```

#### Pull Request Process

1. Update your fork:
   ```bash
   git fetch upstream
   git checkout main
   git merge upstream/main
   ```
2. Rebase your feature branch:
   ```bash
   git checkout feature/your-feature-name
   git rebase main
   ```
3. Push to your fork:
   ```bash
   git push origin feature/your-feature-name
   ```
4. Create Pull Request with:
   - Clear title and description
   - Link to related issue(s)
   - Summary of changes
   - Test results
   - Screenshots (if UI changes)

#### PR Checklist

- [ ] Tests pass locally (`make test`)
- [ ] Code follows Go conventions
- [ ] Comments added for exported functions
- [ ] Documentation updated
- [ ] No security vulnerabilities
- [ ] Performance impact considered
- [ ] Backward compatibility maintained

### Code Style

#### Go Guidelines

1. Follow [Effective Go](https://golang.org/doc/effective_go.html)
2. Use meaningful variable names
3. Keep functions small and focused
4. Handle errors explicitly
5. Add comments for complex logic

#### Example:

```go
// ProcessRequest handles incoming WebSocket requests and routes them
// to the appropriate handler based on the estimated processing duration.
// Short requests (<5s) are processed synchronously, while longer requests
// are queued for async processing.
func (r *Router) ProcessRequest(ctx context.Context, req *Request) (*Response, error) {
    // Validate request
    if err := r.validateRequest(req); err != nil {
        return nil, fmt.Errorf("invalid request: %w", err)
    }
    
    // Find handler
    handler, exists := r.handlers[req.Action]
    if !exists {
        return nil, ErrHandlerNotFound
    }
    
    // Route based on duration
    if handler.EstimatedDuration() <= syncThreshold {
        return r.processSync(ctx, handler, req)
    }
    
    return r.queueAsync(ctx, req)
}
```

### Testing

#### Test Requirements

- Unit tests for all new functions
- Integration tests for new features
- Benchmark tests for performance-critical code
- Table-driven tests preferred

#### Test Example:

```go
func TestRouter_ProcessRequest(t *testing.T) {
    tests := []struct {
        name    string
        request *Request
        wantErr bool
    }{
        {
            name: "valid sync request",
            request: &Request{
                Action: "echo",
                Payload: map[string]interface{}{"message": "test"},
            },
            wantErr: false,
        },
        {
            name: "unknown action",
            request: &Request{
                Action: "unknown",
            },
            wantErr: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            router := setupTestRouter(t)
            _, err := router.ProcessRequest(context.Background(), tt.request)
            if (err != nil) != tt.wantErr {
                t.Errorf("ProcessRequest() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### Documentation

#### When to Update Docs

- New features or APIs
- Changed behavior
- New configuration options
- Better examples needed

#### Documentation Standards

1. Clear and concise
2. Include code examples
3. Explain why, not just how
4. Keep it up to date

### Review Process

1. **Automated checks** run on all PRs
2. **Code review** by maintainers
3. **Testing** in staging environment
4. **Approval** from at least one maintainer
5. **Merge** using squash and merge

### Getting Help

- Check [documentation](https://github.com/pay-theory/streamer/tree/main/docs)
- Ask in [GitHub Discussions](https://github.com/pay-theory/streamer/discussions)
- Join our Slack channel (coming soon)
- Email: opensource@paytheory.com

## Recognition

Contributors will be:
- Listed in [CONTRIBUTORS.md](CONTRIBUTORS.md)
- Mentioned in release notes
- Given credit in relevant documentation

## License

By contributing, you agree that your contributions will be licensed under the Apache 2.0 License.

Thank you for contributing to Streamer! 🚀 