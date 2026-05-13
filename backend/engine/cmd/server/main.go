package main

import (
	"log"
	"os"

	"omnimarket-engine/internal/api"
	"omnimarket-engine/internal/db"
	"omnimarket-engine/internal/matching"
	"omnimarket-engine/internal/ws"

	"github.com/joho/godotenv"
)

func main() {
	// Load .env file if it exists
	_ = godotenv.Load()

	// Initialize Database Connection
	if err := db.ConnectDB(); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.CloseDB()

	// Initialize WebSocket Hub
	hub := ws.NewHub()
	go hub.Run()

	// Initialize Matching Engine
	engine := matching.NewMatchingEngine(hub)

	// Setup Router
	router := api.NewRouter(engine, hub)
	app := router.SetupRoutes()

	// Start Server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Starting OmniMarket Trading Engine on port %s", port)
	if err := app.Run(":" + port); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
