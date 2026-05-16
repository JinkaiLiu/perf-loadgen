# Contributing

## Development

```bash
git clone https://github.com/JinkaiLiu/vibeready.git
cd perf-loadgen

# Build
go build ./cmd/...

# Test
go test ./...
go test -count=1 ./... -timeout 60s

# Benchmarks
go test -bench=. ./internal/stats ./internal/engine

# Lint
go vet ./...
```

Requires Go >= 1.25.

## Project Structure

See [README.md](README.md#architecture) for the architecture diagram and package responsibilities.

## Adding a New Protocol

1. Create a new package under `internal/protocol/<name>/`.
2. Implement the `runner.Runner` interface:
   ```go
   type Runner interface {
       Run(ctx context.Context, req types.RequestSpec) (types.RunResult, error)
   }
   ```
3. Call `registry.Register("your-protocol", factory)` in an `init()` function.
4. Add a blank import in `internal/protocol/factory.go`.
5. Add validation in `internal/config/config.go` for any new config fields.
6. Add CLI flags in `internal/cli/parse.go`.
7. Write tests with a mock server.

## Before Submitting a PR

- `go test -count=1 ./...` must pass
- `go vet ./...` must produce no output
- New code should have tests
- Update README.md if adding user-facing features
