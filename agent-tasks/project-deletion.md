# Project Deletion: Backend-Only Plan

## Scope
- Add backend support to soft-delete a project via a new API endpoint.
- UI work is out of scope (lives in a separate repo).

## Current State
- Backend exposes GET, PUT for projects; no DELETE yet (`backend/bootstrap/main.go`).
- Model `Project` uses `gorm.Model` (has `DeletedAt`) so soft-delete is supported by default.

## API
- Method: DELETE
- Path: `/api/projects/:project_id/`
- Headers: same as existing projects endpoints (Authorization, DIGGER_ORG_ID, DIGGER_USER_ID, DIGGER_ORG_SOURCE).
- Responses: 204 on success; 404 not found; 403 forbidden; 500 on error.

## Backend Steps
1) Route: In `backend/bootstrap/main.go`, register `projectsApiGroup.DELETE("/:project_id/", controllers.DeleteProjectApi)` inside the `if enableApi` block.
2) Controller: In `backend/controllers/projects.go`, add `DeleteProjectApi`:
   - Resolve org from headers (`ORGANISATION_ID_KEY`, `ORGANISATION_SOURCE_KEY`).
   - Load org by `external_id` + `external_source`.
   - Load project by `projects.organisation_id = org.ID AND projects.id = :project_id`.
   - Soft delete via `models.DB.GormDB.Delete(&project)` and return `204`.
3) Migrations: none (soft-delete already present and indexed).

## Out of Scope
- Any UI wiring or server functions in the UI repo.

## Security
- Endpoint is protected by existing API middleware (`InternalApiAuth`, `HeadersApiAuth`).

## Acceptance Criteria
- DELETE `/api/projects/:project_id/` returns 204 on success.
- After deletion, `GET /api/projects/` no longer lists the project (GORM soft-delete filtering applies).
- `GET /api/projects/:project_id/` returns 404 for the deleted project.

## Test Plan
- API: DELETE with valid headers → 204; repeat → 404; missing/invalid headers → 403.
- API: Verify project disappears from list endpoint; details endpoint returns 404 post-delete.

## Estimate
- Backend: 1–2 hours.

## Pointers
- Routes: `backend/bootstrap/main.go`
- Controller: `backend/controllers/projects.go`
- Model: `backend/models/orgs.go`
