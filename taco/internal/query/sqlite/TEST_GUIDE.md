# Test Suite Guide - Quick Reference

## Overview

This directory contains comprehensive tests for the SQLite query backend with RBAC integration. All tests use **mock data** (no external dependencies required).

## 📊 Test Suites Summary

| Test Suite | File | Test Count | Duration | Purpose |
|-----------|------|------------|----------|---------|
| **RBAC Integration** | `rbac_integration_test.go` | 12 tests | ~0.02s | Full RBAC stack testing |
| **Initialization Modes** | `initialization_test.go` | 6 tests | ~0.04s | RBAC setup scenarios |
| **Query Store** | `query_store_test.go` | 13 tests | ~0.05s | Direct query backend testing |
| **Versioning & Management** | `versioning_and_management_test.go` | 14 tests | ~0.38s | Version ops + RBAC.manage |
| **Query Concurrency** | `query_store_test.go` | 1 test | ~0.01s | Concurrent read operations |

**Total: 46 test cases, ~0.5 seconds**

---

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

### Expected Output
```
PASS: TestRBACIntegration (0.02s)
PASS: TestInitializationModes (0.04s)
PASS: TestQueryStore (0.05s)
PASS: TestVersioningAndManagement (0.38s)
PASS: TestQueryStoreConcurrency (0.01s)
PASS
ok  	github.com/diggerhq/digger/opentaco/internal/query/sqlite	0.691s
```

---

## 📋 Test Suite Details

### 1. RBAC Integration Tests

**File:** `rbac_integration_test.go`  
**Purpose:** Tests the complete RBAC enforcement stack with SQLite query backend

#### What It Tests
- ✅ RBAC initialization creates default roles and admin user
- ✅ Admin users have full access to all units
- ✅ Reader role can only read units
- ✅ Writer role can read and write but not delete
- ✅ Wildcard permissions (`*`, `dev/*`) work correctly
- ✅ Prefix-based permissions are enforced
- ✅ Unauthorized access is blocked
- ✅ List operations return only authorized units
- ✅ Multiple roles accumulate permissions
- ✅ Lock operations respect permissions
- ✅ Missing principal returns unauthorized
- ✅ Users with no roles have no access

#### Run This Suite
```bash
# Run all RBAC integration tests
go test -v -run TestRBACIntegration

# Run specific test
go test -v -run TestRBACIntegration/admin_user_has_full_access

# Run with detailed SQL logging
go test -v -run TestRBACIntegration 2>&1 | grep "SELECT\|INSERT\|UPDATE"
```

#### Example Test
```go
// Tests that admin users can perform all operations
func testAdminFullAccess(t *testing.T, env *testEnvironment)
```

---

### 2. Initialization Mode Tests

**File:** `initialization_test.go`  
**Purpose:** Tests various RBAC initialization scenarios

#### What It Tests
- ✅ First-time initialization succeeds
- ✅ Re-initialization is idempotent (can run multiple times)
- ✅ Initialization without RBAC works
- ✅ Initialization with RBAC enabled works
- ✅ Query store syncs RBAC data correctly
- ✅ Late RBAC initialization (after system running)

#### Run This Suite
```bash
# Run all initialization tests
go test -v -run TestInitializationModes

# Run specific initialization scenario
go test -v -run TestInitializationModes/first_time_initialization
```

#### Example Scenarios
- Fresh system with no RBAC → Initialize → Admin created
- Existing system → Re-initialize → No errors, data preserved
- System running → Enable RBAC → Retroactively applied

---

### 3. Query Store Tests

**File:** `query_store_test.go`  
**Purpose:** Tests SQLite query backend **without** authorization layer

#### What It Tests
- ✅ Basic unit CRUD operations
- ✅ RBAC query methods (CanPerformAction, ListUnitsForUser)
- ✅ Permission syncing to database
- ✅ Role syncing to database
- ✅ User syncing to database
- ✅ List units filtered by user permissions
- ✅ Can perform action queries (with pattern matching)
- ✅ Pattern matching in SQL queries
- ✅ Filter unit IDs by user
- ✅ Has RBAC roles check
- ✅ Unit locking operations
- ✅ SQL view creation and querying
- ✅ Concurrent read operations (20 goroutines)

#### Run This Suite
```bash
# Run all query store tests
go test -v -run TestQueryStore

# Run specific query test
go test -v -run "TestQueryStore/pattern_matching"

# Run concurrency test
go test -v -run TestQueryStoreConcurrency
```

#### Example Test
```go
// Tests that pattern matching works in SQL queries
func testPatternMatching(t *testing.T)
```

**Key Difference:** These tests bypass the authorization layer to test database queries directly.

---

### 4. Versioning & Management Tests

**File:** `versioning_and_management_test.go`  
**Purpose:** Tests version operations and RBAC management permissions

#### What It Tests

**Version Operations (5 tests):**
- ✅ Users with `unit.read` can list versions
- ✅ Users without `unit.read` get forbidden
- ✅ Users with `unit.write` can restore versions
- ✅ Users without `unit.write` cannot restore
- ✅ Pattern permissions work for version operations
- ✅ Version operations respect locks

**RBAC Management (3 tests):**
- ✅ `rbac.manage` permission is enforced
- ✅ Admin role includes `rbac.manage` by default
- ✅ Non-admin users cannot modify RBAC

**Pattern Edge Cases (5 tests):**
- ✅ Deep nesting: `org/team/env/*`
- ✅ Special characters: `app-name-v2/*`
- ✅ Multiple segments: `myapp/*/database`
- ✅ Global wildcard: `*`
- ✅ Root namespace: `myapp/*`

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

#### Example Tests
```go
// Tests that restore requires write permission
func testRestoreVersionRequiresWritePermission(t *testing.T)

// Tests that rbac.manage is enforced
func testRBACManagePermissionEnforcement(t *testing.T)
```

---

## 🎯 Running Specific Tests

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

---

## 🐛 Debugging Tests

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

---

## 📊 Test Data Flow

### RBAC Integration Tests
```
Test Code
  ↓
Initialize RBAC
  ↓
Create Permissions/Roles/Users → Saved to Mock S3 (JSON files)
  ↓
Sync to Query Store → Saved to SQLite
  ↓
Create Units → Saved to Mock Blob Store (in-memory)
  ↓
Test Authorization → Query SQLite + Check Blob Store
  ↓
Assert Results
```

### Query Store Tests
```
Test Code
  ↓
Create RBAC Data Directly in SQLite
  ↓
Execute SQL Queries
  ↓
Assert Query Results
```

---

## 🔍 Common Test Patterns

### Pattern 1: Setup Test Environment
```go
env := setupTestEnvironment(t)
defer env.cleanup()
```

### Pattern 2: Create User with Permissions
```go
setupUserWithPermission(t, env.queryStore, "user@example.com", "dev/*", 
    []rbac.Action{rbac.ActionUnitRead})
```

### Pattern 3: Add Principal to Context
```go
ctx = storage.ContextWithPrincipal(ctx, principal.Principal{
    Subject: "user@example.com",
    Email:   "user@example.com",
})
```

### Pattern 4: Test Permission Denied
```go
_, err := env.authStore.Download(ctx, "prod/app1")
assert.Error(t, err)
assert.Contains(t, err.Error(), "forbidden")
```

### Pattern 5: Test Permission Allowed
```go
data, err := env.authStore.Download(ctx, "dev/app1")
require.NoError(t, err)
assert.NotNil(t, data)
```

---

## ⚙️ Test Configuration

### Environment Variables
```bash
# Set max versions for version tests
export OPENTACO_MAX_VERSIONS=10

# Run tests with environment
OPENTACO_MAX_VERSIONS=5 go test -v -run TestVersioning
```

### Timeout
```bash
# Increase timeout for slower machines
go test -timeout 10m -v
```

### Parallel Execution
```bash
# Run tests in parallel (default)
go test -v -parallel 4

# Run tests sequentially (for debugging)
go test -v -parallel 1
```

---

## 📈 Performance

### Benchmark Query Performance
```bash
# Run benchmarks (if available)
go test -bench=. -benchmem

# Profile CPU usage
go test -cpuprofile=cpu.prof -run TestRBACIntegration
go tool pprof cpu.prof
```

### Test Execution Times
```
TestRBACIntegration        ~0.02s  (12 tests)
TestInitializationModes    ~0.04s  (6 tests)
TestQueryStore             ~0.05s  (13 tests)
TestVersioningAndMgmt      ~0.38s  (14 tests)
TestQueryStoreConcurrency  ~0.01s  (1 test)
─────────────────────────────────────────────
Total                      ~0.50s  (46 tests)
```

---

## ✅ CI/CD Integration

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

---

## 🎓 Understanding Test Output

### Successful Test
```
=== RUN   TestRBACIntegration
=== RUN   TestRBACIntegration/admin_user_has_full_access
--- PASS: TestRBACIntegration (0.02s)
    --- PASS: TestRBACIntegration/admin_user_has_full_access (0.00s)
```

### Failed Test
```
=== RUN   TestRBACIntegration/unauthorized_access_is_blocked
    rbac_integration_test.go:450: 
        Error:      Expected error to contain "forbidden"
        Actual:     got nil error
--- FAIL: TestRBACIntegration/unauthorized_access_is_blocked (0.00s)
```

### With SQL Logging
```
2025/10/09 15:03:38 /path/to/sql_store.go:224
[0.027ms] [rows:1] 
SELECT COALESCE(MAX(CASE WHEN r.effect = 'allow' THEN 1 ELSE 0 END), 0)
FROM users u
JOIN user_roles ur ON u.id = ur.user_id
...
```

---

## 📚 Related Documentation

- **Architecture**: `MOCK_DATA_ARCHITECTURE.md` - Detailed mock system explanation
- **Coverage Analysis**: `TEST_COVERAGE_ANALYSIS.md` - What we test vs documentation
- **Missing Tests**: `MISSING_TESTS_SUMMARY.md` - Gaps and future work
- **Pattern Fix**: `PATTERN_MATCHING_FIX.md` - Wildcard pattern implementation
- **Test Suite README**: `TEST_SUITE_README.md` - Original test suite documentation

---

## 🆘 Troubleshooting

### Test Hangs
```bash
# Add timeout and kill hanging tests
go test -timeout 30s -v -run TestRBACIntegration
```

### Database Locked Error
```bash
# This is expected in concurrent write tests for SQLite
# For SQLite, only read concurrency is tested
# Write concurrency requires PostgreSQL backend
```

### Permission Denied Errors
```bash
# Check that user has been assigned the correct role
# Check that role has the correct permission
# Check that permission has the correct rule
```

### "User not found" Errors
```bash
# Ensure user was synced to query store:
env.queryStore.SyncUser(ctx, userAssignment)
```

---

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

---

## 📝 Notes

- All tests use **temporary directories** - no manual cleanup needed
- Tests are **isolated** - can run in any order
- Tests use **mock data** - no external dependencies
- **SQLite** is the only real component - everything else is mocked
- Tests clean up automatically on success or failure
- Safe to run in CI/CD pipelines

---

**Last Updated:** October 2025  
**Test Count:** 46 tests across 5 suites  
**Status:** ✅ All tests passing

