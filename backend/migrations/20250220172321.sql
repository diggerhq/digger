-- Create index "idx_user_external_source" to table: "users"
CREATE UNIQUE INDEX "idx_user_external_source" ON "public"."users" ("external_source", "external_id");
