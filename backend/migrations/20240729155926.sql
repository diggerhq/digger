-- Modify "job_artefacts" table
ALTER TABLE "public"."job_artefacts" ADD COLUMN "size" bigint NULL, ADD COLUMN "content_type" text NULL;
