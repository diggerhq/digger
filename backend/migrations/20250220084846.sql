-- Modify "users" table
ALTER TABLE "public"."users" ADD COLUMN "email" text NULL,
ADD COLUMN "external_id" text NULL,
ADD COLUMN "org_id" bigint NULL;
