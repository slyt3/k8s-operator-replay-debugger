# Contributing to K8s Operator Replay Debugger

Thank you for your interest in contributing!

## Code Standards

This project follows the **JPL Power of 10** rules for safety-critical code:

1. No recursion
2. All loops must be bounded
3. No dynamic memory allocation after init
4. Functions under 60 lines
5. Minimum 2 assertions per function
6. Minimal variable scope
7. All return values checked
8. Limited preprocessor use
9. Single-level pointer dereferencing
10. Zero compiler warnings

See SAFETY_COMPLIANCE.md for details.

## Development Workflow

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/amazing-feature`
3. Make your changes following the coding standards
4. Run tests: `make test`
5. Run linter: `make lint`
6. Commit: `git commit -m "Add amazing feature"`
7. Push: `git push origin feature/amazing-feature`
8. Open a Pull Request

## Testing Requirements

All PRs must include:
- Unit tests for new functionality
- All existing tests passing
- Zero compiler warnings
- Compliance with safety-critical standards

## Areas for Improvement

See [Issues](https://github.com/YOUR_USERNAME/k8s-operator-replay-debugger/issues) for current priorities.
