# Copilot Instructions for Cartographer-Agent

## Project Overview

Cartographer-Agent is a lightweight system inventory tool written in Go. It runs on servers to collect various system information and reports them to a centralized server via REST API. The agent helps maintain a real-time map of infrastructure.

## Tech Stack

- **Language**: Go 1.23.2
- **Key Dependencies**:
  - `github.com/go-co-op/gocron/v2` - Task scheduling
  - `github.com/google/uuid` - UUID generation
  - `github.com/zcalusic/sysinfo` - System information collection
  - `gopkg.in/yaml.v3` - YAML configuration parsing

## Project Structure

- `/main.go` - Entry point for the agent
- `/configuration/` - Configuration management and validation
- `/internal/` - Core agent logic
  - `/internal/agent.go` - Main agent orchestration
  - `/internal/collectors/` - System information collectors (disk usage, users, auth logs, Nessus, etc.)
  - `/internal/report.go` - Reporting to the server
  - `/internal/heartbeat.go` - Heartbeat functionality
  - `/internal/selfupdate.go` - Agent self-update capability
- `/common/` - Common utilities and helpers
- `/config-example.yaml` - Example configuration file

## Build, Test, and Lint Commands

The project uses a Makefile for common operations:

- **Testing**: `go test ./... -v` or `make test`
- **Building**: `make build` (builds for multiple platforms: Linux amd64/arm, Darwin amd64/arm64)
- **Linting**: `make lint` (uses `go vet` and `golint`)
- **Formatting**: `make format` (uses `go fmt` and `goimports`)
- **All checks**: `make all` (pulls tags, formats, lints, tests, and builds)
- **Development setup**: `make dev-setup` (installs golint and goimports)

## Coding Standards and Conventions

1. **Code Style**: Follow standard Go conventions (gofmt, goimports)
2. **Error Handling**: Always check and handle errors appropriately
3. **Logging**: Use the standard library `log/slog` package
4. **Testing**: Write unit tests for new functionality in `*_test.go` files
5. **Comments**: Add meaningful comments for exported functions and complex logic
6. **Configuration**: Support both CLI flags and YAML configuration file

## Development Workflow

1. **Make Changes**: Edit code following Go best practices
2. **Format**: Run `make format` to format code
3. **Lint**: Run `make lint` to check for issues
4. **Test**: Run `make test` to ensure tests pass
5. **Build**: Run `make build` to verify cross-platform compilation

## Key Considerations

- The agent is designed to run on multiple platforms (Linux and macOS)
- Cross-compilation support is essential for all platforms
- Configuration can be provided via CLI flags or YAML file
- The agent can run once or as a daemon
- Collectors gather different types of system information independently
- The agent has self-update capability
- Always maintain backward compatibility with configuration format

## Common Tasks

- **Adding a new collector**: Create a new file in `/internal/collectors/` implementing the collector interface, add tests, and register in `getcollectors.go`
- **Modifying configuration**: Update `/configuration/config.go` and validation logic
- **Updating dependencies**: Use `go mod tidy` and verify all tests still pass
- **Testing locally**: Use `--dryrun` flag to test without sending data to a server

## CI/CD

GitHub Actions workflows are located in `.github/workflows/`:
- `tests.yaml` - Runs tests on every push
- `release.yaml` - Handles releases

## Important Notes

- Always test on multiple platforms when making significant changes
- Keep the agent lightweight and efficient
- Maintain clear error messages for troubleshooting
- Configuration validation happens before the agent runs
- The agent supports version display with `--version` flag
