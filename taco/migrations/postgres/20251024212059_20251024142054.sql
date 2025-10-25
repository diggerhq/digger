-- Create "permissions" table
CREATE TABLE "public"."permissions" (
  "id" character varying(36) NOT NULL,
  "org_id" character varying(36) NULL,
  "name" character varying(255) NOT NULL,
  "description" text NULL,
  "created_by" text NULL,
  "created_at" timestamptz NULL,
  PRIMARY KEY ("id")
);
-- Create index "idx_permissions_name" to table: "permissions"
CREATE INDEX "idx_permissions_name" ON "public"."permissions" ("name");
-- Create index "idx_permissions_org_id" to table: "permissions"
CREATE INDEX "idx_permissions_org_id" ON "public"."permissions" ("org_id");
-- Create "organizations" table
CREATE TABLE "public"."organizations" (
  "id" character varying(36) NOT NULL,
  "name" character varying(255) NOT NULL,
  "display_name" character varying(255) NOT NULL,
  "created_by" character varying(255) NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  PRIMARY KEY ("id")
);
-- Create index "idx_organizations_name" to table: "organizations"
CREATE UNIQUE INDEX "idx_organizations_name" ON "public"."organizations" ("name");
-- Create "roles" table
CREATE TABLE "public"."roles" (
  "id" character varying(36) NOT NULL,
  "org_id" character varying(36) NULL,
  "name" character varying(255) NOT NULL,
  "description" text NULL,
  "created_at" timestamptz NULL,
  "created_by" text NULL,
  PRIMARY KEY ("id")
);
-- Create index "idx_roles_name" to table: "roles"
CREATE INDEX "idx_roles_name" ON "public"."roles" ("name");
-- Create index "idx_roles_org_id" to table: "roles"
CREATE INDEX "idx_roles_org_id" ON "public"."roles" ("org_id");
-- Create "role_permissions" table
CREATE TABLE "public"."role_permissions" (
  "role_id" character varying(36) NOT NULL,
  "permission_id" character varying(36) NOT NULL,
  PRIMARY KEY ("role_id", "permission_id"),
  CONSTRAINT "fk_role_permissions_permission" FOREIGN KEY ("permission_id") REFERENCES "public"."permissions" ("id") ON UPDATE CASCADE ON DELETE CASCADE,
  CONSTRAINT "fk_role_permissions_role" FOREIGN KEY ("role_id") REFERENCES "public"."roles" ("id") ON UPDATE CASCADE ON DELETE CASCADE
);
-- Create index "idx_role_permissions_permission_id" to table: "role_permissions"
CREATE INDEX "idx_role_permissions_permission_id" ON "public"."role_permissions" ("permission_id");
-- Create index "idx_role_permissions_role_id" to table: "role_permissions"
CREATE INDEX "idx_role_permissions_role_id" ON "public"."role_permissions" ("role_id");
-- Create "rules" table
CREATE TABLE "public"."rules" (
  "id" character varying(36) NOT NULL,
  "permission_id" character varying(36) NOT NULL,
  "effect" character varying(8) NOT NULL DEFAULT 'allow',
  "wildcard_action" boolean NOT NULL DEFAULT false,
  "wildcard_resource" boolean NOT NULL DEFAULT false,
  "resource_patterns" text NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_permissions_rules" FOREIGN KEY ("permission_id") REFERENCES "public"."permissions" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create index "idx_rules_permission_id" to table: "rules"
CREATE INDEX "idx_rules_permission_id" ON "public"."rules" ("permission_id");
-- Create "rule_actions" table
CREATE TABLE "public"."rule_actions" (
  "id" character varying(36) NOT NULL,
  "rule_id" character varying(36) NOT NULL,
  "action" character varying(128) NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_rules_actions" FOREIGN KEY ("rule_id") REFERENCES "public"."rules" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create index "idx_rule_actions_action" to table: "rule_actions"
CREATE INDEX "idx_rule_actions_action" ON "public"."rule_actions" ("action");
-- Create index "idx_rule_actions_rule_id" to table: "rule_actions"
CREATE INDEX "idx_rule_actions_rule_id" ON "public"."rule_actions" ("rule_id");
-- Create "rule_unit_tags" table
CREATE TABLE "public"."rule_unit_tags" (
  "id" character varying(36) NOT NULL,
  "rule_id" character varying(36) NOT NULL,
  "tag_id" character varying(36) NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_rules_tag_targets" FOREIGN KEY ("rule_id") REFERENCES "public"."rules" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create index "idx_rule_unit_tags_rule_id" to table: "rule_unit_tags"
CREATE INDEX "idx_rule_unit_tags_rule_id" ON "public"."rule_unit_tags" ("rule_id");
-- Create index "idx_rule_unit_tags_tag_id" to table: "rule_unit_tags"
CREATE INDEX "idx_rule_unit_tags_tag_id" ON "public"."rule_unit_tags" ("tag_id");
-- Create "rule_units" table
CREATE TABLE "public"."rule_units" (
  "id" character varying(36) NOT NULL,
  "rule_id" character varying(36) NOT NULL,
  "unit_id" character varying(36) NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_rules_unit_targets" FOREIGN KEY ("rule_id") REFERENCES "public"."rules" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create index "idx_rule_units_rule_id" to table: "rule_units"
CREATE INDEX "idx_rule_units_rule_id" ON "public"."rule_units" ("rule_id");
-- Create index "idx_rule_units_unit_id" to table: "rule_units"
CREATE INDEX "idx_rule_units_unit_id" ON "public"."rule_units" ("unit_id");
-- Create "tags" table
CREATE TABLE "public"."tags" (
  "id" character varying(36) NOT NULL,
  "org_id" character varying(36) NULL,
  "name" character varying(255) NOT NULL,
  PRIMARY KEY ("id")
);
-- Create index "idx_tags_name" to table: "tags"
CREATE INDEX "idx_tags_name" ON "public"."tags" ("name");
-- Create index "idx_tags_org_id" to table: "tags"
CREATE INDEX "idx_tags_org_id" ON "public"."tags" ("org_id");
-- Create "units" table
CREATE TABLE "public"."units" (
  "id" character varying(36) NOT NULL,
  "org_id" character varying(36) NULL,
  "name" character varying(255) NOT NULL,
  "size" bigint NULL DEFAULT 0,
  "updated_at" timestamptz NULL,
  "locked" boolean NULL DEFAULT false,
  "lock_id" text NULL DEFAULT '',
  "lock_who" text NULL DEFAULT '',
  "lock_created" timestamptz NULL,
  PRIMARY KEY ("id")
);
-- Create index "idx_units_name" to table: "units"
CREATE INDEX "idx_units_name" ON "public"."units" ("name");
-- Create index "idx_units_org_id" to table: "units"
CREATE INDEX "idx_units_org_id" ON "public"."units" ("org_id");
-- Create "unit_tags" table
CREATE TABLE "public"."unit_tags" (
  "unit_id" character varying(36) NOT NULL,
  "tag_id" character varying(36) NOT NULL,
  PRIMARY KEY ("unit_id", "tag_id"),
  CONSTRAINT "fk_unit_tags_tag" FOREIGN KEY ("tag_id") REFERENCES "public"."tags" ("id") ON UPDATE CASCADE ON DELETE CASCADE,
  CONSTRAINT "fk_unit_tags_unit" FOREIGN KEY ("unit_id") REFERENCES "public"."units" ("id") ON UPDATE CASCADE ON DELETE CASCADE
);
-- Create index "idx_unit_tags_tag_id" to table: "unit_tags"
CREATE INDEX "idx_unit_tags_tag_id" ON "public"."unit_tags" ("tag_id");
-- Create index "idx_unit_tags_unit_id" to table: "unit_tags"
CREATE INDEX "idx_unit_tags_unit_id" ON "public"."unit_tags" ("unit_id");
-- Create "users" table
CREATE TABLE "public"."users" (
  "id" character varying(36) NOT NULL,
  "subject" character varying(255) NOT NULL,
  "email" character varying(255) NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "version" bigint NULL,
  PRIMARY KEY ("id")
);
-- Create index "idx_users_email" to table: "users"
CREATE UNIQUE INDEX "idx_users_email" ON "public"."users" ("email");
-- Create index "idx_users_subject" to table: "users"
CREATE UNIQUE INDEX "idx_users_subject" ON "public"."users" ("subject");
-- Create "user_roles" table
CREATE TABLE "public"."user_roles" (
  "user_id" character varying(36) NOT NULL,
  "role_id" character varying(36) NOT NULL,
  "org_id" character varying(36) NOT NULL,
  PRIMARY KEY ("user_id", "role_id", "org_id"),
  CONSTRAINT "fk_user_roles_role" FOREIGN KEY ("role_id") REFERENCES "public"."roles" ("id") ON UPDATE CASCADE ON DELETE CASCADE,
  CONSTRAINT "fk_user_roles_user" FOREIGN KEY ("user_id") REFERENCES "public"."users" ("id") ON UPDATE CASCADE ON DELETE CASCADE
);
-- Create index "idx_user_roles_org_id" to table: "user_roles"
CREATE INDEX "idx_user_roles_org_id" ON "public"."user_roles" ("org_id");
-- Create index "idx_user_roles_role_id" to table: "user_roles"
CREATE INDEX "idx_user_roles_role_id" ON "public"."user_roles" ("role_id");
-- Create index "idx_user_roles_user_id" to table: "user_roles"
CREATE INDEX "idx_user_roles_user_id" ON "public"."user_roles" ("user_id");
