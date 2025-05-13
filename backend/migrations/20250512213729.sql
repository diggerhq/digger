-- Modify "digger_batches" table
ALTER TABLE "public"."digger_batches" ADD COLUMN "created_at" timestamptz NULL, ADD COLUMN "updated_at" timestamptz NULL, ADD COLUMN "deleted_at" timestamptz NULL;
-- Create index "idx_digger_batches_deleted_at" to table: "digger_batches"
CREATE INDEX "idx_digger_batches_deleted_at" ON "public"."digger_batches" ("deleted_at");
