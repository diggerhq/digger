-- Create "digger_locks" table
CREATE TABLE "public"."digger_locks" (
  "id" bigserial NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "resource" text NULL,
  "lock_id" bigint NULL,
  "organisation_id" bigint NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_digger_locks_organisation" FOREIGN KEY ("organisation_id") REFERENCES "public"."organisations" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "idx_digger_locked_resource" to table: "digger_locks"
CREATE INDEX "idx_digger_locked_resource" ON "public"."digger_locks" ("resource");
-- Create index "idx_digger_locks_deleted_at" to table: "digger_locks"
CREATE INDEX "idx_digger_locks_deleted_at" ON "public"."digger_locks" ("deleted_at");
