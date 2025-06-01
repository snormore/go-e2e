# Containerized testing tool for Go

[![Checks](https://github.com/snormore/go-e2e/actions/workflows/checks.yaml/badge.svg)](https://github.com/snormore/go-e2e/actions/workflows/checks.yaml)

A tool for running Go tests in isolated containers with parallel execution.

## Features

- Containerized test execution
- Parallel test runs
- Recursive config file discovery
- Configurable test patterns
- YAML configuration

## Installation

Add to your `go.mod`:

```go
tool (
    github.com/snormore/go-e2e
)
```

Or install directly:

```bash
go install github.com/snormore/go-e2e@latest
```

## Usage

The tool will automatically find `e2e.yaml` config files in the current directory and its subdirectories, and execute containerized tests in parallel for each.

1. Create an `e2e.yaml` configuration file in a directory with your end-to-end tests.

```yaml
dockerfile: Dockerfile
docker-run-args: []
```

2. Create a `Dockerfile` for your tests:

```dockerfile
FROM golang:1.24.3-alpine AS builder
WORKDIR /work
COPY . .
RUN go test -c -o /bin/your-test.test -tags e2e

FROM ubuntu:22.04
RUN apt-get update && apt-get install -y ca-certificates
WORKDIR /work
COPY --from=builder /bin/your-test.test /bin/
ENTRYPOINT ["/bin/your-test.test"]
CMD ["-test.v"]
```

3. Run your tests using either:

```bash
# Using go-e2e directly
go-e2e

# Or using go tool
go tool go-e2e
```

## Command Line Options

- `-f`: Path to config file (default: e2e.yaml)
- `-v`, `-vv`, `-vvv`: Verbosity level
- `--no-fast-fail`: Run all tests even if one fails
- `--no-parallel`: Run tests sequentially
- `-p`, `--parallelism`: Number of tests to run in parallel (default: number of CPUs)

## License

[MIT License](LICENSE)
