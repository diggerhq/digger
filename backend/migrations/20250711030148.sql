-- Modify "organisations" table
ALTER TABLE "public"."organisations" ADD COLUMN "drift_webhook_url" text NULL, ADD COLUMN "drift_cron_tab" text NULL DEFAULT '0 0 * * *';
