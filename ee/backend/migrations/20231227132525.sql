-- Create "digger_job_parent_links" table
CREATE TABLE "public"."digger_job_parent_links" (
  "id" bigserial NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "digger_job_id" text NULL,
  "parent_digger_job_id" text NULL,
  PRIMARY KEY ("id")
);
-- Create index "idx_digger_job_parent_links_deleted_at" to table: "digger_job_parent_links"
CREATE INDEX "idx_digger_job_parent_links_deleted_at" ON "public"."digger_job_parent_links" ("deleted_at");
-- Create "github_app_installations" table
CREATE TABLE "public"."github_app_installations" (
  "id" bigserial NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "github_installation_id" bigint NULL,
  "github_app_id" bigint NULL,
  "account_id" bigint NULL,
  "login" text NULL,
  "repo" text NULL,
  "status" bigint NULL,
  PRIMARY KEY ("id")
);
-- Create index "idx_github_app_installations_deleted_at" to table: "github_app_installations"
CREATE INDEX "idx_github_app_installations_deleted_at" ON "public"."github_app_installations" ("deleted_at");
-- Create "github_apps" table
CREATE TABLE "public"."github_apps" (
  "id" bigserial NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "github_id" bigint NULL,
  "name" text NULL,
  "github_app_url" text NULL,
  PRIMARY KEY ("id")
);
-- Create index "idx_github_apps_deleted_at" to table: "github_apps"
CREATE INDEX "idx_github_apps_deleted_at" ON "public"."github_apps" ("deleted_at");
-- Create "github_digger_job_links" table
CREATE TABLE "public"."github_digger_job_links" (
  "id" bigserial NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "digger_job_id" text NULL,
  "repo_full_name" text NULL,
  "github_job_id" bigint NULL,
  "github_workflow_run_id" bigint NULL,
  "status" smallint NULL,
  PRIMARY KEY ("id")
);
-- Create index "idx_github_digger_job_links_deleted_at" to table: "github_digger_job_links"
CREATE INDEX "idx_github_digger_job_links_deleted_at" ON "public"."github_digger_job_links" ("deleted_at");
-- Create index "idx_github_job_id" to table: "github_digger_job_links"
CREATE INDEX "idx_github_job_id" ON "public"."github_digger_job_links" ("github_job_id");
-- Create "digger_batches" table
CREATE TABLE "public"."digger_batches" (
  "id" text NOT NULL,
  "pr_number" bigint NULL,
  "status" smallint NULL,
  "branch_name" text NULL,
  "digger_config" text NULL,
  "github_installation_id" bigint NULL,
  "repo_full_name" text NULL,
  "repo_owner" text NULL,
  "repo_name" text NULL,
  "batch_type" text NULL,
  PRIMARY KEY ("id")
);
-- Create "digger_jobs" table
CREATE TABLE "public"."digger_jobs" (
  "id" bigserial NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "digger_job_id" text NULL,
  "status" smallint NULL,
  "batch_id" text NULL,
  "serialized_job" bytea NULL,
  "status_updated_at" timestamptz NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_digger_jobs_batch" FOREIGN KEY ("batch_id") REFERENCES "public"."digger_batches" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "idx_digger_job_id" to table: "digger_jobs"
CREATE INDEX "idx_digger_job_id" ON "public"."digger_jobs" ("batch_id");
-- Create index "idx_digger_jobs_deleted_at" to table: "digger_jobs"
CREATE INDEX "idx_digger_jobs_deleted_at" ON "public"."digger_jobs" ("deleted_at");
-- Create "organisations" table
CREATE TABLE "public"."organisations" (
  "id" bigserial NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "name" text NULL,
  "external_source" text NULL,
  "external_id" text NULL,
  PRIMARY KEY ("id")
);
-- Create index "idx_external_source" to table: "organisations"
CREATE UNIQUE INDEX "idx_external_source" ON "public"."organisations" ("external_source", "external_id");
-- Create index "idx_organisation" to table: "organisations"
CREATE UNIQUE INDEX "idx_organisation" ON "public"."organisations" ("name");
-- Create index "idx_organisations_deleted_at" to table: "organisations"
CREATE INDEX "idx_organisations_deleted_at" ON "public"."organisations" ("deleted_at");
-- Create "github_app_installation_links" table
CREATE TABLE "public"."github_app_installation_links" (
  "id" bigserial NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "github_installation_id" bigint NULL,
  "organisation_id" bigint NULL,
  "status" smallint NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_github_app_installation_links_organisation" FOREIGN KEY ("organisation_id") REFERENCES "public"."organisations" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "idx_github_app_installation_links_deleted_at" to table: "github_app_installation_links"
CREATE INDEX "idx_github_app_installation_links_deleted_at" ON "public"."github_app_installation_links" ("deleted_at");
-- Create index "idx_github_installation_org" to table: "github_app_installation_links"
CREATE INDEX "idx_github_installation_org" ON "public"."github_app_installation_links" ("github_installation_id", "organisation_id");
-- Create "users" table
CREATE TABLE "public"."users" (
  "id" bigserial NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "username" text NULL,
  PRIMARY KEY ("id")
);
-- Create index "idx_user" to table: "users"
CREATE UNIQUE INDEX "idx_user" ON "public"."users" ("username");
-- Create index "idx_users_deleted_at" to table: "users"
CREATE INDEX "idx_users_deleted_at" ON "public"."users" ("deleted_at");
-- Create "repos" table
CREATE TABLE "public"."repos" (
  "id" bigserial NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "name" text NULL,
  "organisation_id" bigint NULL,
  "digger_config" text NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_repos_organisation" FOREIGN KEY ("organisation_id") REFERENCES "public"."organisations" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "idx_org_repo" to table: "repos"
CREATE UNIQUE INDEX "idx_org_repo" ON "public"."repos" ("name", "organisation_id");
-- Create index "idx_repos_deleted_at" to table: "repos"
CREATE INDEX "idx_repos_deleted_at" ON "public"."repos" ("deleted_at");
-- Create "projects" table
CREATE TABLE "public"."projects" (
  "id" bigserial NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "name" text NULL,
  "organisation_id" bigint NULL,
  "repo_id" bigint NULL,
  "configuration_yaml" text NULL,
  "status" bigint NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_projects_organisation" FOREIGN KEY ("organisation_id") REFERENCES "public"."organisations" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "fk_projects_repo" FOREIGN KEY ("repo_id") REFERENCES "public"."repos" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "idx_project" to table: "projects"
CREATE UNIQUE INDEX "idx_project" ON "public"."projects" ("name", "organisation_id", "repo_id");
-- Create index "idx_projects_deleted_at" to table: "projects"
CREATE INDEX "idx_projects_deleted_at" ON "public"."projects" ("deleted_at");
-- Create "policies" table
CREATE TABLE "public"."policies" (
  "id" bigserial NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "project_id" bigint NULL,
  "policy" text NULL,
  "type" text NULL,
  "created_by_id" bigint NULL,
  "organisation_id" bigint NULL,
  "repo_id" bigint NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_policies_created_by" FOREIGN KEY ("created_by_id") REFERENCES "public"."users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "fk_policies_organisation" FOREIGN KEY ("organisation_id") REFERENCES "public"."organisations" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "fk_policies_project" FOREIGN KEY ("project_id") REFERENCES "public"."projects" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "fk_policies_repo" FOREIGN KEY ("repo_id") REFERENCES "public"."repos" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "idx_policies_deleted_at" to table: "policies"
CREATE INDEX "idx_policies_deleted_at" ON "public"."policies" ("deleted_at");
-- Create "project_runs" table
CREATE TABLE "public"."project_runs" (
  "id" bigserial NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "project_id" bigint NULL,
  "started_at" bigint NULL,
  "ended_at" bigint NULL,
  "status" text NULL,
  "command" text NULL,
  "output" text NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_project_runs_project" FOREIGN KEY ("project_id") REFERENCES "public"."projects" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "idx_project_runs_deleted_at" to table: "project_runs"
CREATE INDEX "idx_project_runs_deleted_at" ON "public"."project_runs" ("deleted_at");
-- Create "tokens" table
CREATE TABLE "public"."tokens" (
  "id" bigserial NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "value" text NULL,
  "organisation_id" bigint NULL,
  "type" text NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_tokens_organisation" FOREIGN KEY ("organisation_id") REFERENCES "public"."organisations" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "idx_token" to table: "tokens"
CREATE UNIQUE INDEX "idx_token" ON "public"."tokens" ("value");
-- Create index "idx_tokens_deleted_at" to table: "tokens"
CREATE INDEX "idx_tokens_deleted_at" ON "public"."tokens" ("deleted_at");
