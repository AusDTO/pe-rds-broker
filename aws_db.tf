/*
 * Create databases by running `terraform apply`
 * Get a list of database environment variables with `terraform output`
 * Get a list of environment variables that can be exported in a shell with
 *   terraform output | sed 's/ = /=/g; s/^/export /g'
 *
 * For ways of specifying the AWS access credentials, see
 * https://www.terraform.io/docs/providers/aws/index.html
 * Note: This script requires the same AWS access as the broker itself.
 *
 * For ways of specifying all other variables, see
 * https://www.terraform.io/intro/getting-started/variables.html#assigning-variables
 *
 */

variable "region" {
  default = "ap-southeast-2"
}
variable "vpc_security_group_ids" {
  type = "list"
}
variable "db_subnet_group_name" {}
variable "internal_db_password" {}
variable "shared_postgres_password" {}
variable "shared_mysql_password" {}
variable "internaldb_storage" {
  default = 5
}
variable "shared_storage" {
  default = 5
}
variable "internaldb_instance_class" {
  default = "db.t2.micro"
}
variable "shared_instance_class" {
  default = "db.t2.micro"
}
variable "backup_retention_period" {
  default = 10
}

variable "id_prefix" {
  default = "rds-broker"
}

provider "aws" {
  region = "${var.region}"
}

/*
 * For a full list of parameters, see
 * https://www.terraform.io/docs/providers/aws/r/db_instance.html
 */
resource "aws_db_instance" "rds_broker_internaldb" {
  allocated_storage = "${var.internaldb_storage}"
  engine = "postgres"
  engine_version = "9.6"
  allow_major_version_upgrade = true
  instance_class = "${var.internaldb_instance_class}"
  identifier = "${var.id_prefix}-internaldb"
  name = "rds_broker_internaldb"
  username = "rds_broker"
  password = "${var.internal_db_password}"
  vpc_security_group_ids = "${var.vpc_security_group_ids}"
  db_subnet_group_name = "${var.db_subnet_group_name}"
  backup_retention_period = "${var.backup_retention_period}"
}

resource "aws_db_instance" "rds_broker_shared_postgres" {
  allocated_storage = "${var.shared_storage}"
  engine = "postgres"
  engine_version = "9.6"
  allow_major_version_upgrade = true
  instance_class = "${var.shared_instance_class}"
  identifier = "${var.id_prefix}-shared-postgres"
  name = "rds_broker_shared_postgres"
  username = "rds_broker"
  password = "${var.shared_postgres_password}"
  vpc_security_group_ids = "${var.vpc_security_group_ids}"
  db_subnet_group_name = "${var.db_subnet_group_name}"
  backup_retention_period = "${var.backup_retention_period}"
}

resource "aws_db_instance" "rds_broker_shared_mysql" {
  allocated_storage = "${var.shared_storage}"
  engine = "mysql"
  engine_version = "5.7.11"
  allow_major_version_upgrade = true
  instance_class = "${var.shared_instance_class}"
  identifier = "${var.id_prefix}-shared-mysql"
  name = "rds_broker_shared_mysql"
  username = "rds_broker"
  password = "${var.shared_mysql_password}"
  vpc_security_group_ids = "${var.vpc_security_group_ids}"
  db_subnet_group_name = "${var.db_subnet_group_name}"
  backup_retention_period = "${var.backup_retention_period}"
}

output RDSBROKER_INTERNAL_DB_PROVIDER {
  value = "postgres"
}
output RDSBROKER_INTERNAL_DB_NAME {
  value = "${aws_db_instance.rds_broker_internaldb.name}"
}
output RDSBROKER_INTERNAL_DB_URL {
  value = "${aws_db_instance.rds_broker_internaldb.address}"
}
output RDSBROKER_INTERNAL_DB_PORT {
  value = "${aws_db_instance.rds_broker_internaldb.port}"
}
output RDSBROKER_INTERNAL_DB_USERNAME {
  value = "${aws_db_instance.rds_broker_internaldb.username}"
}
output RDSBROKER_INTERNAL_DB_PASSWORD {
  value = "${aws_db_instance.rds_broker_internaldb.password}"
}
output RDSBROKER_INTERNAL_DB_SSLMODE {
  value = "require"
}
output RDSBROKER_SHARED_POSTGRES_DB_NAME {
  value = "${aws_db_instance.rds_broker_shared_postgres.name}"
}
output RDSBROKER_SHARED_POSTGRES_DB_URL {
  value = "${aws_db_instance.rds_broker_shared_postgres.address}"
}
output RDSBROKER_SHARED_POSTGRES_DB_PORT {
  value = "${aws_db_instance.rds_broker_shared_postgres.port}"
}
output RDSBROKER_SHARED_POSTGRES_DB_USERNAME {
  value = "${aws_db_instance.rds_broker_shared_postgres.username}"
}
output RDSBROKER_SHARED_POSTGRES_DB_PASSWORD {
  value = "${aws_db_instance.rds_broker_shared_postgres.password}"
}
output RDSBROKER_SHARED_POSTGRES_DB_SSLMODE {
  value = "require"
}
output RDSBROKER_SHARED_MYSQL_DB_NAME {
  value = "${aws_db_instance.rds_broker_shared_mysql.name}"
}
output RDSBROKER_SHARED_MYSQL_DB_URL {
  value = "${aws_db_instance.rds_broker_shared_mysql.address}"
}
output RDSBROKER_SHARED_MYSQL_DB_PORT {
  value = "${aws_db_instance.rds_broker_shared_mysql.port}"
}
output RDSBROKER_SHARED_MYSQL_DB_USERNAME {
  value = "${aws_db_instance.rds_broker_shared_mysql.username}"
}
output RDSBROKER_SHARED_MYSQL_DB_PASSWORD {
  value = "${aws_db_instance.rds_broker_shared_mysql.password}"
}
output RDSBROKER_SHARED_MYSQL_DB_SSLMODE {
  value = "require"
}
