package main

import (
	"log"
	"os"

	"github.com/user/gpoptimizer/server/auth"
	"github.com/user/gpoptimizer/server/handler"
	"github.com/user/gpoptimizer/server/relay"
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

	addr := os.Getenv("SERVER_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	r := relay.NewMemory()
	router := handler.NewRouter(db, r, auth.ConfigFromEnv())

	if err := router.Run(addr); err != nil {
		log.Fatalf("server: %v", err)
	}
}
