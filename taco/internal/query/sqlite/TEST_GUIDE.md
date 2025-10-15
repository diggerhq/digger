# Test Suite Guide - Quick Reference


## 🏃 Quick Start - Run All Tests

### One-Liner (from anywhere in repo)
```bash
# Run all SQLite query tests
cd taco/internal/query/sqlite && go test -v

# Or from repo root:
go test -v ./taco/internal/query/sqlite/...

# Quick pass/fail (no verbose):
cd taco/internal/query/sqlite && go test

# With coverage:
cd taco/internal/query/sqlite && go test -cover

# With race detection:
cd taco/internal/query/sqlite && go test -race -v
```


### 1. RBAC Integration Tests

#### Run This Suite
```bash
# Run all RBAC integration tests
go test -v -run TestRBACIntegration

# Run specific test
go test -v -run TestRBACIntegration/admin_user_has_full_access

# Run with detailed SQL logging
go test -v -run TestRBACIntegration 2>&1 | grep "SELECT\|INSERT\|UPDATE"
```


---

### 2. Initialization Mode Tests

#### Run This Suite
```bash
# Run all initialization tests
go test -v -run TestInitializationModes

# Run specific initialization scenario
go test -v -run TestInitializationModes/first_time_initialization
```

### 3. Query Store Tests

#### Run This Suite
```bash
# Run all query store tests
go test -v -run TestQueryStore

# Run specific query test
go test -v -run "TestQueryStore/pattern_matching"

# Run concurrency test
go test -v -run TestQueryStoreConcurrency
```

### 4. Versioning & Management Tests


#### Run This Suite
```bash
# Run all versioning and management tests
go test -v -run TestVersioningAndManagement

# Run only version operations
go test -v -run "TestVersioningAndManagement/version_operations"

# Run only RBAC management tests
go test -v -run "TestVersioningAndManagement/rbac.manage"

# Run only pattern matching edge cases
go test -v -run "TestVersioningAndManagement/pattern_matching"
```


## Running Specific Tests

### By Test Suite
```bash
go test -v -run TestRBACIntegration
go test -v -run TestInitializationModes
go test -v -run TestQueryStore
go test -v -run TestVersioningAndManagement
```

### By Test Name
```bash
# Run tests matching a pattern
go test -v -run "wildcard"                    # All tests with "wildcard" in name
go test -v -run "pattern_matching"            # All pattern matching tests
go test -v -run "admin"                       # All admin-related tests
go test -v -run "version"                     # All version-related tests
```

### By Functionality
```bash
# Permission tests
go test -v -run "permission"

# Role tests
go test -v -run "role"

# Lock tests
go test -v -run "lock"

# Initialization tests
go test -v -run "initialization"
```

##  Debugging Tests

### Run with SQL Query Logging
```bash
# See all SQL queries executed
go test -v -run TestRBACIntegration 2>&1 | grep -E "SELECT|INSERT|UPDATE|DELETE"

# See only SELECT queries
go test -v -run TestQueryStore 2>&1 | grep "SELECT"
```

### Run Single Test with Debug
```bash
# Run one specific test with full output
go test -v -run TestRBACIntegration/wildcard_permissions -count=1
```

### Check Test Coverage
```bash
# Generate coverage report
go test -cover -coverprofile=coverage.out

# View coverage in browser
go tool cover -html=coverage.out
```

### Run Tests Multiple Times (Flakiness Check)
```bash
# Run 10 times to check for flaky tests
go test -run TestVersioningAndManagement -count=10

# Run with race detector
go test -race -run TestRBACIntegration
```

## Performance

### Benchmark Query Performance
```bash
# Run benchmarks (if available)
go test -bench=. -benchmem

# Profile CPU usage
go test -cpuprofile=cpu.prof -run TestRBACIntegration
go tool pprof cpu.prof
```

##  CI/CD Integration

### GitHub Actions Example
```yaml
- name: Run SQLite Query Tests
  run: |
    cd taco/internal/query/sqlite
    go test -v -race -cover -timeout 5m
```

### Pre-commit Hook
```bash
#!/bin/bash
# .git/hooks/pre-commit
cd taco/internal/query/sqlite
go test -short || exit 1
```

### Test Hangs
```bash
# Add timeout and kill hanging tests
go test -timeout 30s -v -run TestRBACIntegration
```

## 🎯 Quick Command Reference

### Run All Tests (Most Common)
```bash
# From sqlite directory (RECOMMENDED)
cd taco/internal/query/sqlite && go test -v

# From repo root
go test -v ./taco/internal/query/sqlite/...

# Quick pass/fail only
cd taco/internal/query/sqlite && go test

# With coverage report
cd taco/internal/query/sqlite && go test -cover

# With race detection
cd taco/internal/query/sqlite && go test -race -v
```

### Run Specific Test Suites
```bash
# Run specific suite
cd taco/internal/query/sqlite && go test -v -run TestRBACIntegration
cd taco/internal/query/sqlite && go test -v -run TestVersioningAndManagement

# Run tests matching pattern
cd taco/internal/query/sqlite && go test -v -run "wildcard"
cd taco/internal/query/sqlite && go test -v -run "pattern"
```

### Debugging & Analysis
```bash
# Run and watch for changes (requires entr)
cd taco/internal/query/sqlite && ls *.go | entr -c go test -v

# Show SQL queries during tests
cd taco/internal/query/sqlite && go test -v 2>&1 | grep "SELECT"

# Clean test cache (if tests behave oddly)
go clean -testcache

# Verbose output with paging
cd taco/internal/query/sqlite && go test -v 2>&1 | less

# Count passing tests
cd taco/internal/query/sqlite && go test -v 2>&1 | grep -c "PASS:"

# Show only failures
cd taco/internal/query/sqlite && go test -v 2>&1 | grep -E "FAIL|Error"

# Check for flaky tests (run 10 times)
cd taco/internal/query/sqlite && go test -run TestVersioningAndManagement -count=10
```

### Coverage & Reporting
```bash
# Generate coverage report
cd taco/internal/query/sqlite && go test -coverprofile=coverage.out

# View coverage in browser
cd taco/internal/query/sqlite && go test -coverprofile=coverage.out && go tool cover -html=coverage.out

# Coverage percentage only
cd taco/internal/query/sqlite && go test -cover | grep coverage
```

### One-Liner Copy-Paste Commands
```bash
# Ultimate test command (everything you need)
cd /Users/brianreardon/development/digger/taco/internal/query/sqlite && go test -v -race -cover

# Quick verification (for git pre-commit)
cd /Users/brianreardon/development/digger/taco/internal/query/sqlite && go test

# CI/CD command
cd /Users/brianreardon/development/digger/taco/internal/query/sqlite && go test -v -race -cover -timeout 5m
```




