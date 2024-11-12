-- Modify "github_app_connections" table
ALTER TABLE "public"."github_app_connections" ADD COLUMN "organisation_id" bigint NULL, ADD
 CONSTRAINT "fk_github_app_connections_organisation" FOREIGN KEY ("organisation_id") REFERENCES "public"."organisations" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
