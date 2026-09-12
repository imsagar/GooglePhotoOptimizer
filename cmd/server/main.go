package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"

	"github.com/user/gpoptimizer/server/auth"
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

	r := gin.Default()
	auth.Routes(r, db, auth.ConfigFromEnv())
	// remaining handlers added in Task 5+

	if err := r.Run(addr); err != nil {
		log.Fatalf("server: %v", err)
	}
}
