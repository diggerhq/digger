variable "DB_URL" {
  type    = string
  default = "postgres://postgres:postgres@localhost:5432/token_service?sslmode=disable"
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

data "external_schema" "gorm_postgres" {
  program = [
    "sh",
    "-c",
    "cd query && go run ./atlas_loader.go postgres",
  ]
}

env "postgres" {
  src = data.external_schema.gorm_postgres.url
  url = var.DB_URL

  dev = "docker://postgres/16?timeout=5m"

  schemas = ["public"]

  migration {
    dir    = var.POSTGRES_MIGRATIONS_DIR
    format = "atlas"
  }

  lint {
    destructive { error = true }
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
    "sh",
    "-c",
    "cd query && go run ./atlas_loader.go mysql",
  ]
}

env "mysql" {
  src = data.external_schema.gorm_mysql.url
  
  dev = "docker://mysql/8/token_service?timeout=5m"
  
  migration {
    dir    = var.MYSQL_MIGRATIONS_DIR
    format = "atlas"
  }
  
  lint {
    destructive { error = true }
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
    "sh",
    "-c",
    "cd query && go run ./atlas_loader.go sqlite",
  ]
}

env "sqlite" {
  src = data.external_schema.gorm_sqlite.url
  
  # SQLite uses in-memory dev database, no Docker needed
  dev = "sqlite://file?mode=memory"
  
  migration {
    dir    = var.SQLITE_MIGRATIONS_DIR
    format = "atlas"
  }
  
  lint {
    destructive { error = true }
  }
  
  format {
    migrate {
      diff = "{{ sql . \"  \" }}"
    }
  }
}

