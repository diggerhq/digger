-- Modify "organisations" table
ALTER TABLE "public"."organisations" ADD COLUMN "billing_plan" text NULL DEFAULT 'free', ADD COLUMN "billing_stripe_subscription_id" text NULL;
