-- Drop index "idx_organizations_name" from table: "organizations"
DROP INDEX "public"."idx_organizations_name";
-- Create index "idx_organizations_name" to table: "organizations"
CREATE INDEX "idx_organizations_name" ON "public"."organizations" ("name");
