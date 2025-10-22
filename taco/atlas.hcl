# SQLite configuration
data "external_schema" "gorm_sqlite" {
  program = [
    "sh",
    "-c",
    "cd internal && go run ./atlas_loader.go",
  ]
}

env "sqlite" {
  src = data.external_schema.gorm_sqlite.url
  dev = "sqlite://file?mode=memory"
  migration {
    dir = "file://internal/query/migration/atlas/migrations/sqlite"
  }
  format {
    migrate {
      diff = "{{ sql . \"  \" }}"
    }
  }
}

# PostgreSQL configuration
data "external_schema" "gorm_postgres" {
  program = [
    "atlas-provider-gorm",
    "load",
    "--path", "./internal/query/types",
    "--dialect", "postgres",
  ]
}

env "postgres" {
  src = data.external_schema.gorm_postgres.url
  dev = "docker://postgres/16.1"
  migration {
    dir = "file://internal/query/migration/atlas/migrations/postgres"
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
    "atlas-provider-gorm",
    "load",
    "--path", "./internal/query/types",
    "--dialect", "mysql",
  ]
}

env "mysql" {
  src = data.external_schema.gorm_mysql.url
  dev = "docker://mysql/8"
  migration {
    dir = "file://internal/query/migration/atlas/migrations/mysql"
  }
  format {
    migrate {
      diff = "{{ sql . \"  \" }}"
    }
  }
}

# SQL Server configuration
data "external_schema" "gorm_sqlserver" {
  program = [
    "atlas-provider-gorm",
    "load",
    "--path", "./internal/query/types",
    "--dialect", "sqlserver",
  ]
}

env "sqlserver" {
  src = data.external_schema.gorm_sqlserver.url
  dev = "docker://sqlserver/2022-latest"
  migration {
    dir = "file://internal/query/migration/atlas/migrations/sqlserver"
  }
  format {
    migrate {
      diff = "{{ sql . \"  \" }}"
    }
  }
}
