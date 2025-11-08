-- Create "digger_detection_runs" table (append-only)
CREATE TABLE "public"."digger_detection_runs" (
  "id" bigserial NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,

  "organisation_id"  bigint       NOT NULL,
  "repo_full_name"   text         NOT NULL,
  "pr_number"        integer      NOT NULL,

  -- What triggered this detection
  "trigger_type"     text         NOT NULL, -- 'pull_request' | 'issue_comment'
  "trigger_action"   text         NOT NULL, -- e.g. opened | synchronize | reopened | comment | closed | converted_to_draft

  -- Context
  "commit_sha"       text,
  "default_branch"   text,
  "target_branch"    text,

  -- Denormalized JSON payloads
  "labels_json"              jsonb,
  "changed_files_json"       jsonb,
  "impacted_projects_json"   jsonb NOT NULL,
  "source_mapping_json"      jsonb,

  PRIMARY KEY ("id")
);

-- Helpful indexes for lookups and listing latest runs per PR
CREATE INDEX IF NOT EXISTS idx_ddr_org_repo_pr_created_at
  ON "public"."digger_detection_runs" ("organisation_id", "repo_full_name", "pr_number", "created_at" DESC);

CREATE INDEX IF NOT EXISTS idx_ddr_repo_pr
  ON "public"."digger_detection_runs" ("repo_full_name", "pr_number");

CREATE INDEX IF NOT EXISTS idx_ddr_deleted_at
  ON "public"."digger_detection_runs" ("deleted_at");

