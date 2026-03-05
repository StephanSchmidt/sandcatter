# Testing Guide

Sandcatter includes comprehensive unit and integration tests to ensure reliability.

## Test Structure

```
pkg/
├── dockerfile/
│   ├── modifier.go          # Dockerfile manipulation
│   ├── modifier_test.go     # Tests for Dockerfile modification
│   ├── compose.go           # Compose file manipulation
│   └── compose_test.go      # Tests for compose file modification
└── plugin/
    ├── plugin.go            # Plugin loading and validation
    └── plugin_test.go       # Tests for plugin system
```

## Running Tests

### Quick Start

```bash
# Run all tests
make test

# Run with coverage
make test-coverage

# Run integration tests
make integration-test

# Run everything (fmt, vet, test)
make check
```

### Individual Test Commands

```bash
# Unit tests only
go test ./pkg/...

# Verbose output
go test -v ./pkg/...

# With coverage
go test -cover ./pkg/...

# Specific package
go test ./pkg/dockerfile
go test ./pkg/plugin
```

## Test Coverage

Current coverage (as of last run):
- **pkg/dockerfile**: 86.2% coverage
- **pkg/plugin**: 9.9% coverage (mostly validation logic)

View detailed coverage:
```bash
make test-coverage
# Opens coverage.html in your browser
```

## What's Tested

### Dockerfile Manipulation Tests

**TestAddAptPackages**
- Adding packages to simple apt command
- Adding packages to multi-line apt command
- Skipping already installed packages
- Idempotency (no changes when all packages exist)

**TestHasPackage**
- Detecting installed packages
- Avoiding false positives (e.g., "vi" vs "vim")
- Multi-line apt install commands

**TestAddLocaleSetup**
- Adding locale generation commands
- Setting environment variables (LANG, LANGUAGE, LC_ALL)
- Idempotency (not re-adding if already present)

**TestAddCopyCommand**
- Inserting COPY commands before ENTRYPOINT
- Setting file permissions
- Idempotency (not duplicating COPY commands)

### Compose File Tests

**TestAddEnvironmentVariables**
- Creating new environment section when missing
- Adding to existing environment section
- Idempotency (not duplicating variables)
- Error handling for missing app service
- Empty variable map handling

### Plugin Tests

**TestPluginValidation**
- Valid plugin manifests
- Missing required fields (name, version)
- Invalid file mappings (missing source/destination)
- File existence checks

**TestPluginGetFilePath**
- Path resolution relative to plugin directory

## Integration Testing

The integration test (`test.sh`) performs end-to-end testing:

1. Extracts fresh sandcat from tarball
2. Applies tmux plugin
3. Verifies Dockerfile changes
4. Verifies compose file changes
5. Tests idempotency (running twice)
6. Checks for duplicates

Run with:
```bash
make integration-test
# or
./test.sh
```

## Test Infrastructure

### Afero Filesystem Abstraction

Tests use [afero](https://github.com/spf13/afero) for in-memory filesystem testing:

```go
// Create in-memory filesystem
fs := afero.NewMemMapFs()
afero.WriteFile(fs, "/Dockerfile", []byte(content), 0644)

// Load and test
df, err := LoadFs(fs, "/Dockerfile")
```

**Benefits:**
- Fast tests (no disk I/O)
- Isolated (no side effects)
- Parallel execution safe
- Easy cleanup

### Test Fixtures

Tests use inline fixture data for clarity:

```go
input := `FROM debian
RUN apt-get update \
    && apt-get install -y vim \
    && rm -rf /var/lib/apt/lists/*`
```

This makes tests self-contained and easy to understand.

## Writing New Tests

### Dockerfile Manipulation Tests

```go
func TestMyFeature(t *testing.T) {
    input := `... Dockerfile content ...`

    fs := afero.NewMemMapFs()
    afero.WriteFile(fs, "/Dockerfile", []byte(input), 0644)

    df, err := LoadFs(fs, "/Dockerfile")
    if err != nil {
        t.Fatalf("Failed to load: %v", err)
    }

    // Test your feature
    err = df.MyFeature(params)
    if err != nil {
        t.Fatalf("MyFeature failed: %v", err)
    }

    // Verify results
    got := df.GetContent()
    if !strings.Contains(got, "expected content") {
        t.Error("Expected content not found")
    }
}
```

### Compose File Tests

```go
func TestComposeFeature(t *testing.T) {
    input := `... compose.yml content ...`

    fs := afero.NewMemMapFs()
    afero.WriteFile(fs, "/compose.yml", []byte(input), 0644)

    cf, err := LoadComposeFs(fs, "/compose.yml")
    if err != nil {
        t.Fatalf("Failed to load: %v", err)
    }

    // Test and verify...
}
```

### Table-Driven Tests

Use table-driven tests for multiple scenarios:

```go
tests := []struct {
    name     string
    input    string
    expected string
}{
    {
        name:     "scenario 1",
        input:    "...",
        expected: "...",
    },
    // More test cases...
}

for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        // Run test with tt.input and tt.expected
    })
}
```

## Continuous Integration

To add CI (GitHub Actions example):

```yaml
name: Tests
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      - run: make check
      - run: make integration-test
```

## Test Data

### Fresh Sandcat Tarball

`fresh-sandcat.tar.gz` contains a pristine sandcat installation for testing:

```bash
# Recreate if needed
cd test-sandcat
bash install.sh --name test-project
cd ..
tar czf fresh-sandcat.tar.gz test-sandcat/.devcontainer test-sandcat/install.sh
```

## Debugging Failed Tests

### Verbose Output

```bash
go test -v ./pkg/dockerfile/
```

### Single Test

```bash
go test -v ./pkg/dockerfile/ -run TestAddAptPackages
```

### With Race Detection

```bash
go test -race ./...
```

### Debug Failing Assertion

```go
got := df.GetContent()
want := expectedContent
if got != want {
    t.Errorf("Mismatch:\nGot:\n%s\n\nWant:\n%s", got, want)
}
```

## Best Practices

1. **Test both success and failure cases**
   - Valid input
   - Invalid input
   - Edge cases

2. **Test idempotency**
   - Running twice should produce same result
   - No duplicate entries

3. **Use descriptive test names**
   ```go
   func TestAddAptPackages_WithExistingPackages_SkipsDuplicates(t *testing.T)
   ```

4. **Keep tests focused**
   - One concept per test
   - Use subtests for variations

5. **Use table-driven tests for variations**
   - Reduces duplication
   - Makes adding cases easy

6. **Test actual behavior, not implementation**
   - Test what the code does
   - Not how it does it

## Troubleshooting

### Tests fail with "permission denied"
- Ensure using afero.NewMemMapFs() for tests
- Check file permissions in test fixtures

### Integration test fails
- Verify fresh-sandcat.tar.gz exists
- Check sandcatter binary is built
- Run with bash -x ./test.sh for debugging

### Coverage report not generating
- Ensure go tool cover is available
- Check coverage.out was created
- Try: `go tool cover -html=coverage.out`

## Future Improvements

- [ ] Increase plugin package coverage
- [ ] Add benchmarks for large Dockerfiles
- [ ] Test error recovery scenarios
- [ ] Add fuzzing tests for parser
- [ ] Test with real sandcat containers
- [ ] Performance tests for large plugin sets
