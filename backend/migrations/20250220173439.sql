-- Rename a column from "org_id" to "organisation_id"
ALTER TABLE "public"."users" RENAME COLUMN "org_id" TO "organisation_id";
-- Modify "users" table
ALTER TABLE "public"."users" ADD CONSTRAINT "fk_users_organisation" FOREIGN KEY ("organisation_id") REFERENCES "public"."organisations" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
