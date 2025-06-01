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

```go.mod
tool github.com/snormore/go-e2e
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

```
  -f string
        Config filename to search for recursively (default: e2e.yaml) (default "e2e.yaml")
  -help
        Show help
  -no-fast-fail
        Run all tests even if one fails (default: false)
  -no-parallel
        Run tests sequentially instead of in parallel (default: false)
  -p int
        Number of tests to run in parallel (default: number of CPUs) (default 10)
  -parallelism int
        Number of tests to run in parallel (default: number of CPUs) (default 10)
  -run string
        Run only tests matching the pattern (default: all tests)
  -verbose int
        Verbosity level (default: 0)
```

### Example

```
$ go tool go-e2e

=== Running tests from e2e.yaml ===
--- INFO: Building docker image e2e-test-runner-d4c6:dev (this may take a while)...
--- OK: docker build (0.43s)
--- INFO: Running 2 tests in parallel (max 10)...
=== RUN: TestExample2
=== RUN: TestExample1
--- PASS: TestExample1 (0.19s)
--- PASS: TestExample2 (0.20s)

=== SUMMARY: PASS (0.20s)
PASS: TestExample1 (0.19s)
PASS: TestExample2 (0.20s)
```

## License

[MIT License](LICENSE)
