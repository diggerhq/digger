-- Modify "repos" table
ALTER TABLE "public"."repos" ADD COLUMN "vcs" text NULL DEFAULT 'github';
