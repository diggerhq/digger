package query 


import ( 
	"os"
	"strings"
	"time"
	"github.com/diggerhq/digger/opentaco/internal/query/sqlite"
	"github.com/diggerhq/digger/opentaco/internal/query/noop"
)



func NewQueryStoreFromEnv() (QueryStore, error) {

	backend := os.Getenv("TACO_QUERY_BACKEND")
	backend = strings.ToLower(backend) // lowercase everythign


	switch backend {
		case "sqlite":
			return newSQLiteFromEnv()
		case "off":
			return noop.NewNoOpQueryStore(), nil 
		default: 
			return newSQLiteFromEnv() 
	}
}


func newSQLiteFromEnv() (QueryStore, error) {
    cfg := sqlite.Config{
        Path:              getEnv("TACO_SQLITE_PATH", "./data/taco.db"),
        Cache:             getEnv("TACO_SQLITE_CACHE", "shared"),
        EnableForeignKeys: getEnvBool("TACO_SQLITE_FOREIGN_KEYS", true),
        EnableWAL:         getEnvBool("TACO_SQLITE_WAL", true),
        BusyTimeout:       5 * time.Second,
        MaxOpenConns:      1,
        MaxIdleConns:      1,
        ConnMaxLifetime:   0,
    }
    
    return sqlite.NewSQLiteQueryStore(cfg)
}

func getEnv(key, defaultVal string) string {
    if val := os.Getenv(key); val != "" {
        return val
    }
    return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
    if val := os.Getenv(key); val != "" {
        return val == "true" || val == "1"
    }
    return defaultVal
}