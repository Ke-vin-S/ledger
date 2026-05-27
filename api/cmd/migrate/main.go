package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	m, err := migrate.New("file://internal/db/migrations", dbURL)
	if err != nil {
		log.Fatalf("create migrator: %v", err)
	}
	defer func() {
		srcErr, dbErr := m.Close()
		if srcErr != nil {
			log.Printf("close source: %v", srcErr)
		}
		if dbErr != nil {
			log.Printf("close db: %v", dbErr)
		}
	}()

	args := os.Args[1:]
	if len(args) == 0 {
		log.Fatal("usage: migrate <up|down|version> [steps]")
	}

	switch args[0] {
	case "up":
		if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			log.Fatalf("migrate up: %v", err)
		}
		v, _, _ := m.Version()
		fmt.Printf("migrated up — version %d\n", v)

	case "down":
		steps := 1
		if len(args) > 1 {
			n, err := strconv.Atoi(args[1])
			if err != nil || n < 1 {
				log.Fatalf("steps must be a positive integer, got %q", args[1])
			}
			steps = n
		}
		if err := m.Steps(-steps); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			log.Fatalf("migrate down: %v", err)
		}
		v, _, _ := m.Version()
		fmt.Printf("rolled back %d step(s) — version %d\n", steps, v)

	case "version":
		v, dirty, err := m.Version()
		if err != nil {
			log.Fatalf("version: %v", err)
		}
		fmt.Printf("version %d (dirty: %v)\n", v, dirty)

	default:
		log.Fatalf("unknown command %q — use up, down, or version", args[0])
	}
}
