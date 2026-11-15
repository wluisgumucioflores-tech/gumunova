// Command migrate applies the pending database migrations.
package main

import (
	"fmt"
	"os"

	"gumunova/backend/internal/adapters/outbound/postgres"
	"gumunova/backend/internal/config"
	"gumunova/backend/migrations"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		os.Exit(1)
	}
}

func run() error {
	databaseURL, err := config.LoadDatabaseURL()
	if err != nil {
		return err
	}
	if err := postgres.MigrateUp(databaseURL, migrations.Files); err != nil {
		return err
	}
	fmt.Println("migrations are up to date")
	return nil
}
