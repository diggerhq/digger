-- Create index "idx_digger_lock_id" to table: "digger_locks"
CREATE INDEX "idx_digger_lock_id" ON "public"."digger_locks" ("lock_id", "organisation_id");
