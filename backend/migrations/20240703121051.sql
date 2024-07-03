-- Modify "projects" table
ALTER TABLE "public"."projects" ADD COLUMN "is_generated" boolean NULL, ADD COLUMN "is_in_main_branch" boolean NULL;
