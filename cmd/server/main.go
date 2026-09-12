package main

import (
	"log"
	"os"

	"github.com/user/gpoptimizer/server/store"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL required")
	}
	db, err := store.New(dbURL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer db.Close()
	log.Println("connected to database")
	// handlers added in Task 3+
	select {}
}
