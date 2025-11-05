# Project Deletion: Implementation Plan (Essentials)

## Scope
- Add a Delete Project action on the Project Details screen.
- Add backend support to soft-delete a project.

## Current State
- UI files: `projects.$projectid.tsx`, `projects.index.tsx`.
- UI API: `UI/src/api/orchestrator_projects.ts` has get/update; no delete.
- ServerFns: `UI/src/api/orchestrator_serverFunctions.ts` has `getProjectFn`, `getProjectsFn`, `updateProjectFn`.
- Backend routes: `backend/bootstrap/main.go` exposes GET, PUT for projects; no DELETE.
- Model: `Project` uses `gorm.Model` (soft-delete supported).

## API
- Method: DELETE
- Path: `/api/projects/:project_id/`
- Headers: same as existing projects endpoints (Authorization, DIGGER_ORG_ID, DIGGER_USER_ID, DIGGER_ORG_SOURCE).
- Responses: 204 on success; 404 not found; 403 forbidden; 500 on error.

## Backend Steps
1) Add route in `backend/bootstrap/main.go`:
   - `projectsApiGroup.DELETE("/:project_id/", controllers.DeleteProjectApi)`
2) Implement `DeleteProjectApi` in `backend/controllers/projects.go`:
   - Resolve org from headers; fetch project by `org.ID` and `project_id`.
   - `models.DB.GormDB.Delete(&project)` to soft-delete.
   - Return 204.
3) Migrations: none (soft-delete already present).

## UI Steps
1) Add `deleteProject` to `UI/src/api/orchestrator_projects.ts` (DELETE request with existing headers). Return empty object on 204.
2) Add `deleteProjectFn` to `UI/src/api/orchestrator_serverFunctions.ts` wrapping `deleteProject`.
3) Update `projects.$projectid.tsx`:
   - Import `useRouter`, `useToast`, `deleteProjectFn`, and `AlertDialog` components; add a destructive Delete button.
   - On confirm: call `deleteProjectFn({ data: { projectId, organisationId, userId } })`, toast success, navigate to `/dashboard/projects`.
   - On error: toast destructive.

## Security
- Endpoint protected by existing API middleware; UI invokes via server function to keep secrets server-side.

## Acceptance Criteria
- Delete button shows on Project Details; confirmation dialog appears.
- Confirm deletes the project (soft-delete), redirects to Projects list, and shows success toast.
- Deleted project no longer appears in list; details by ID returns 404.

## Test Plan
- API: DELETE with valid headers → 204; repeat → 404; missing headers → 403.
- UI: Confirm delete redirects to list; failure shows error toast.

## Estimate
- Backend: 1–2 hours; UI: ~1 hour; total: ~2–3 hours.

## Pointers
- Routes: `backend/bootstrap/main.go`
- Controller: `backend/controllers/projects.go`
- Model: `backend/models/orgs.go`
- UI API: `UI/src/api/orchestrator_projects.ts`
- ServerFn: `UI/src/api/orchestrator_serverFunctions.ts`
- Screen: `UI/src/routes/_authenticated/_dashboard/dashboard/projects.$projectid.tsx`
