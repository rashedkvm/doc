/*
Copyright 2020 The CRDS Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package store

import (
	"fmt"
	"log"
	"os"
)

// New creates a Store based on the given driver name.
//
// Supported drivers:
//   - "sqlite" (default): embedded SQLite database. dsn is the file path.
//   - "postgres": PostgreSQL via pgxpool. dsn is a connection string.
//
// When driver is empty, it defaults to "sqlite".
func New(driver, dsn string) (Store, error) {
	switch driver {
	case "sqlite", "":
		log.Printf("Opening SQLite store at %q", dsn)
		return NewSQLiteStore(dsn)
	case "postgres":
		log.Printf("Opening PostgreSQL store")
		return NewPostgresStore(dsn)
	default:
		return nil, fmt.Errorf("unknown DB driver: %q", driver)
	}
}

// NewFromEnv creates a Store by reading DB_DRIVER and DB_DSN environment
// variables. For backward compatibility, if DB_DRIVER is not set but PG_HOST
// is present, it falls back to postgres mode and builds the DSN from the
// legacy PG_* environment variables.
func NewFromEnv() (Store, error) {
	driver := os.Getenv("DB_DRIVER")
	dsn := os.Getenv("DB_DSN")

	if driver == "" && os.Getenv("PG_HOST") != "" {
		driver = "postgres"
		dsn = fmt.Sprintf("postgresql://%s:%s@%s:%s/%s",
			os.Getenv("PG_USER"),
			os.Getenv("PG_PASS"),
			os.Getenv("PG_HOST"),
			os.Getenv("PG_PORT"),
			os.Getenv("PG_DB"),
		)
		log.Print("DB_DRIVER not set but PG_HOST found; falling back to postgres mode")
	}

	if driver == "" {
		driver = "sqlite"
	}
	if dsn == "" && driver == "sqlite" {
		dsn = "./doc.db"
	}

	return New(driver, dsn)
}
