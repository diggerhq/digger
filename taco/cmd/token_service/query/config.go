package query

import "time"

// Config holds database configuration for the token service
type Config struct {
	Backend  string         `envconfig:"TOKEN_QUERY_BACKEND" default:"sqlite"`
	SQLite   SQLiteConfig   `envconfig:"TOKEN_SQLITE"`
	Postgres PostgresConfig `envconfig:"TOKEN_POSTGRES"`
	MSSQL    MSSQLConfig    `envconfig:"TOKEN_MSSQL"`
	MySQL    MySQLConfig    `envconfig:"TOKEN_MYSQL"`
}

type SQLiteConfig struct {
	Path              string        `envconfig:"DB_PATH" default:"./data/token_service.db"`
	Cache             string        `envconfig:"CACHE" default:"shared"`
	BusyTimeout       time.Duration `envconfig:"BUSY_TIMEOUT" default:"5s"`
	MaxOpenConns      int           `envconfig:"MAX_OPEN_CONNS" default:"25"`
	MaxIdleConns      int           `envconfig:"MAX_IDLE_CONNS" default:"10"`
	PragmaJournalMode string        `envconfig:"PRAGMA_JOURNAL_MODE" default:"WAL"`
	PragmaForeignKeys string        `envconfig:"PRAGMA_FOREIGN_KEYS" default:"ON"`
	PragmaBusyTimeout string        `envconfig:"PRAGMA_BUSY_TIMEOUT" default:"5000"`
}

type PostgresConfig struct {
	Host         string `envconfig:"HOST" default:"localhost"`
	Port         int    `envconfig:"PORT" default:"5432"`
	User         string `envconfig:"USER" default:"postgres"`
	Password     string `envconfig:"PASSWORD"`
	DBName       string `envconfig:"DBNAME" default:"token_service"`
	SSLMode      string `envconfig:"SSLMODE" default:"disable"`
	MaxOpenConns int    `envconfig:"MAX_OPEN_CONNS" default:"25"`
	MaxIdleConns int    `envconfig:"MAX_IDLE_CONNS" default:"10"`
}

type MSSQLConfig struct {
	Host         string `envconfig:"HOST" default:"localhost"`
	Port         int    `envconfig:"PORT" default:"1433"`
	User         string `envconfig:"USER"`
	Password     string `envconfig:"PASSWORD"`
	DBName       string `envconfig:"DBNAME" default:"token_service"`
	MaxOpenConns int    `envconfig:"MAX_OPEN_CONNS" default:"25"`
	MaxIdleConns int    `envconfig:"MAX_IDLE_CONNS" default:"10"`
}

type MySQLConfig struct {
	Host         string `envconfig:"HOST" default:"localhost"`
	Port         int    `envconfig:"PORT" default:"3306"`
	User         string `envconfig:"USER" default:"root"`
	Password     string `envconfig:"PASSWORD"`
	DBName       string `envconfig:"DBNAME" default:"token_service"`
	Charset      string `envconfig:"CHARSET" default:"utf8mb4"`
	MaxOpenConns int    `envconfig:"MAX_OPEN_CONNS" default:"25"`
	MaxIdleConns int    `envconfig:"MAX_IDLE_CONNS" default:"10"`
}

