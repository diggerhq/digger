variable "DB_URL" {
  type    = string
  default = "postgres://postgres:postgres@localhost:5432/devdb?sslmode=disable"
}

variable "POSTGRES_MIGRATIONS_DIR" {
  type    = string
  default = "file://migrations/postgres"
}

variable "MYSQL_MIGRATIONS_DIR" {
  type    = string
  default = "file://migrations/mysql"
}

variable "SQLITE_MIGRATIONS_DIR" {
  type    = string
  default = "file://migrations/sqlite"
}

# Postgres configuration
data "external_schema" "gorm_postgres" {
  program = [
    "go",
    "run",
    "./models/loader",
    "postgres",
  ]
}

env "postgres" {
  src = data.external_schema.gorm_postgres.url
  url = var.DB_URL
  dev = "docker://postgres/16?search_path=public&timeout=5m"
  schemas = ["public"]

  migration {
    dir    = var.POSTGRES_MIGRATIONS_DIR
    format = "atlas"
  }

  lint {
    destructive {
      error = true
    }
  }

  format {
    migrate {
      diff = "{{ sql . \"  \" }}"
    }
  }
}

# MySQL configuration
data "external_schema" "gorm_mysql" {
  program = [
    "go",
    "run",
    "./models/loader",
    "mysql",
  ]
}

env "mysql" {
  src = data.external_schema.gorm_mysql.url
  dev = "docker://mysql/8/devdb?timeout=5m"

  migration {
    dir    = var.MYSQL_MIGRATIONS_DIR
    format = "atlas"
  }

  lint {
    destructive {
      error = true
    }
  }

  format {
    migrate {
      diff = "{{ sql . \"  \" }}"
    }
  }
}

# SQLite configuration
data "external_schema" "gorm_sqlite" {
  program = [
    "go",
    "run",
    "./models/loader",
    "sqlite",
  ]
}

env "sqlite" {
  src = data.external_schema.gorm_sqlite.url
  dev = "sqlite://file?mode=memory"

  migration {
    dir    = var.SQLITE_MIGRATIONS_DIR
    format = "atlas"
  }

  lint {
    destructive {
      error = true
    }
  }

  format {
    migrate {
      diff = "{{ sql . \"  \" }}"
    }
  }
}
