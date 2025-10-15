package query

import "time"


type Config struct {
	Backend  string         `envconfig:"QUERY_BACKEND" default:"sqlite"`
	SQLite   SQLiteConfig   `envconfig:"SQLITE"`
	Postgres PostgresConfig `envconfig:"POSTGRES"`
	MSSQL    MSSQLConfig    `envconfig:"MSSQL"`
	MySQL    MySQLConfig    `envconfig:"MYSQL"`
}


type SQLiteConfig struct { 
	Path              string        `envconfig:"DB_PATH" default:"./data/taco.db"`// if we call it PATH at the struct level, it will pick up the terminal path 
	Cache             string        `envconfig:"CACHE" default:"shared"`
	BusyTimeout       time.Duration `envconfig:"BUSY_TIMEOUT" default:"5s"`
	MaxOpenConns      int           `envconfig:"MAX_OPEN_CONNS" default:"1"`
	MaxIdleConns      int           `envconfig:"MAX_IDLE_CONNS" default:"1"`
	PragmaJournalMode string        `envconfig:"PRAGMA_JOURNAL_MODE" default:"WAL"`
	PragmaForeignKeys string        `envconfig:"PRAGMA_FOREIGN_KEYS" default:"ON"`
	PragmaBusyTimeout string        `envconfig:"PRAGMA_BUSY_TIMEOUT" default:"5000"`
}

type PostgresConfig struct { 
	Host string 	`envconfig:"HOST" default:"localhost"`
	Port int 		`envconfig:"PORT" default:"5432"`
	User string 	`envconfig:"USER" default:"postgres"`
	Password string	`envconfig:"PASSWORD"`
	DBName string 	`envconfig:"DBNAME" default:"taco"` 
	SSLMode string	`envconfig:"SSLMODE" default:"disable"`
}

type MSSQLConfig struct {
	Host     string `envconfig:"HOST" default:"localhost"`
	Port     int    `envconfig:"PORT" default:"1433"`
	User     string `envconfig:"USER"`
	Password string `envconfig:"PASSWORD"`
	DBName   string `envconfig:"DBNAME" default:"taco"`
}

type MySQLConfig struct {
	Host     string `envconfig:"HOST" default:"localhost"`
	Port     int    `envconfig:"PORT" default:"3306"`
	User     string `envconfig:"USER" default:"root"`
	Password string `envconfig:"PASSWORD"`
	DBName   string `envconfig:"DBNAME" default:"taco"`
	Charset  string `envconfig:"CHARSET" default:"utf8mb4"`
}