package query

import "time"

// Config holds all configuration for the query store, loaded from environment variables.
type Config struct {
	Backend string       `envconfig:"QUERY_BACKEND" default:"sqlite"`
	SQLite  SQLiteConfig `envconfig:"SQLITE"`
	// Postgres PostgresConfig `envconfig:"POSTGRES"`
}

// SQLiteConfig holds all the specific settings for the SQLite backend.
type SQLiteConfig struct {
	Path              string        `envconfig:"PATH" default:"./data/taco.db"`
	Cache             string        `envconfig:"CACHE" default:"shared"`
	BusyTimeout       time.Duration `envconfig:"BUSY_TIMEOUT" default:"5s"`
	MaxOpenConns      int           `envconfig:"MAX_OPEN_CONNS" default:"1"`
	MaxIdleConns      int           `envconfig:"MAX_IDLE_CONNS" default:"1"`
	PragmaJournalMode string        `envconfig:"PRAGMA_JOURNAL_MODE" default:"WAL"`
	PragmaForeignKeys string        `envconfig:"PRAGMA_FOREIGN_KEYS" default:"ON"`
	PragmaBusyTimeout string        `envconfig:"PRAGMA_BUSY_TIMEOUT" default:"5000"`
}