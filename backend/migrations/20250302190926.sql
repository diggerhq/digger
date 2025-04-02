-- Modify "digger_batches" table
ALTER TABLE "public"."digger_batches" ADD COLUMN "vcs_connection_id" bigint NULL,
ADD CONSTRAINT "fk_digger_batches_vcs_connection" FOREIGN KEY (
    "vcs_connection_id"
) REFERENCES "public"."github_app_connections" (
    "id"
) ON UPDATE NO ACTION ON DELETE NO ACTION;
