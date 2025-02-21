-- Drop index "idx_organisation" from table: "organisations"
DROP INDEX "public"."idx_organisation";
-- Create index "idx_organisation" to table: "organisations"
CREATE INDEX "idx_organisation" ON "public"."organisations" ("name");
