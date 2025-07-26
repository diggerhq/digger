data "aws_ssm_parameter" "pg_master_password" {
  name = "/${var.env}/rds/pg_master_password"
}

module "pg_db" {
  source = "github.com/terraform-aws-modules/terraform-aws-rds"

  identifier        = "${var.env}-pg-db"
  engine            = "postgres"
  engine_version    = "12.5"
  instance_class    = "db.t3.micro"
  allocated_storage = 20
  storage_encrypted = false

  username = "postgres"
  password = data.aws_ssm_parameter.pg_master_password.value
  port     = "5432"
  multi_az = false

  vpc_security_group_ids = [var.sg_vpc_local_id]

  apply_immediately          = true
  maintenance_window         = "tue:02:13-tue:02:43"
  backup_window              = "05:00-06:00"
  backup_retention_period    = 0
  auto_minor_version_upgrade = true

  # DB subnet group
  subnet_ids = var.public_subnets

  # DB parameter group
  family = "postgres12"

  # DB option group
  major_engine_version = "12"
}

resource "aws_route53_record" "pg_db" {
  zone_id = values(var.dns_zone_id)[0]
  name    = "pg-db.${var.env}.local"
  type    = "CNAME"
  ttl     = "300"
  records = [module.pg_db.this_db_instance_address]
}
