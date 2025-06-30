# Bug Analysis: Issue #2003 - Panic when updating comment in GitHub PR

## Problem Summary
The digger CLI is experiencing a panic with "invalid memory address or nil pointer dereference" when trying to update GitHub PR comments, specifically in terragrunt projects. The panic occurs at `digger.go:162` in the `RunJobs` function.

## Root Cause Analysis

### Stack Trace Analysis
```
panic: runtime error: invalid memory address or nil pointer dereference
[signal SIGSEGV: segmentation violation code=0x1 addr=0x70 pc=0x23921ef]

goroutine 1 [running]:
github.com/diggerhq/digger/cli/pkg/digger.RunJobs
    /home/runner/work/_actions/diggerhq/digger/ec77f5be2d78db212e81656cf7dcf14192947043/cli/pkg/digger/digger.go:162
```

### Exact Problem Location
The panic occurs when the `commentUpdater.UpdateComment()` function is called at line 162 in `digger.go`:

```go
err = commentUpdater.UpdateComment(batchResult.Jobs, prNumber, prService, prCommentId)
```

### Underlying Issue
The bug is in the comment updater implementations that unsafely dereference the `WorkflowRunUrl` pointer without nil checking:

#### Location 1: `/workspace/ee/cli/pkg/comment_updater/updater.go:41`
```go
message = message + fmt.Sprintf("%v **%v** <a href='%v'>%v</a>%v %v\n", 
    job.Status.ToEmoji(), 
    jobSpec.ProjectName, 
    *job.WorkflowRunUrl,  // ← UNSAFE DEREFERENCE
    job.Status.ToString(), 
    job.ResourcesSummaryString(isPlan), 
    DriftSummaryString(job.ProjectName, issuesMap))
```

#### Location 2: `/workspace/libs/comment_utils/summary/updater.go:46`
```go
message = message + fmt.Sprintf("|%v **%v** |<a href='%v'>%v</a> | <a href='%v'>%v</a> | %v | %v | %v|\n",
    job.Status.ToEmoji(),
    jobSpec.ProjectName,
    *job.WorkflowRunUrl,  // ← UNSAFE DEREFERENCE
    job.Status.ToString(),
    // ... more parameters
```

### Why This Happens
1. `WorkflowRunUrl` is defined as `*string` (pointer to string) in the scheduler models
2. For terragrunt projects, the workflow URL detection logic may not always set this field
3. When `WorkflowRunUrl` is nil, dereferencing it with `*job.WorkflowRunUrl` causes the panic
4. The bug affects terragrunt projects specifically because the workflow URL logic differs from regular terraform projects

## Recommended Fix

Add nil safety checks before dereferencing `WorkflowRunUrl`:

### Fix for `/workspace/ee/cli/pkg/comment_updater/updater.go`
```go
workflowUrl := "#"
if job.WorkflowRunUrl != nil {
    workflowUrl = *job.WorkflowRunUrl
}

message = message + fmt.Sprintf("%v **%v** <a href='%v'>%v</a>%v %v\n", 
    job.Status.ToEmoji(), 
    jobSpec.ProjectName, 
    workflowUrl,  // ← SAFE
    job.Status.ToString(), 
    job.ResourcesSummaryString(isPlan), 
    DriftSummaryString(job.ProjectName, issuesMap))
```

### Fix for `/workspace/libs/comment_utils/summary/updater.go`
```go
workflowUrl := "#"
if job.WorkflowRunUrl != nil {
    workflowUrl = *job.WorkflowRunUrl
}

message = message + fmt.Sprintf("|%v **%v** |<a href='%v'>%v</a> | <a href='%v'>%v</a> | %v | %v | %v|\n",
    job.Status.ToEmoji(),
    jobSpec.ProjectName,
    workflowUrl,  // ← SAFE
    job.Status.ToString(),
    // ... rest of parameters
```

## Impact
- **Severity**: High - causes complete failure of digger runs in terragrunt projects
- **Scope**: Affects terragrunt projects specifically when trying to update PR comments
- **Workaround**: None available - the panic terminates the entire process

## Additional Notes
- This appears to be a recent regression, likely introduced when workflow URL functionality was added/modified
- The issue affects both the enterprise (`ee/`) and open-source (`libs/`) comment updater implementations
- The fix is straightforward but needs to be applied consistently across all comment updater implementations

## Files to Modify
1. `/workspace/ee/cli/pkg/comment_updater/updater.go` - Line 41
2. `/workspace/libs/comment_utils/summary/updater.go` - Line 46