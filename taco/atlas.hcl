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

data "external_schema" "gorm_postgres" {
  program = [
    "go",
    "run",
    "-mod=mod",
    "ariga.io/atlas-provider-gorm",
    "load",
    "--path",
    "./internal/query/types",
    "--dialect",
    "postgres",
  ]
}

env "postgres" {
  src = data.external_schema.gorm_postgres.url
  url = var.DB_URL

  # IMPORTANT: no env=… params here; keep 5m timeout.
  # If your build wants fully-qualified, use docker://docker.io/library/postgres/16
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
    "go",
    "run",
    "-mod=mod",
    "ariga.io/atlas-provider-gorm",
    "load",
    "--path",
    "./internal/query/types",
    "--dialect",
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
    "go",
    "run",
    "-mod=mod",
    "ariga.io/atlas-provider-gorm",
    "load",
    "--path",
    "./internal/query/types",
    "--dialect",
    "sqlite",
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
