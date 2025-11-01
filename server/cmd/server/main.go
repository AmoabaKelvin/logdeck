package main

import (
	"log"
	"net/http"

	"github.com/AmoabaKelvin/logdeck/internal/api"
	"github.com/AmoabaKelvin/logdeck/internal/auth"
	"github.com/AmoabaKelvin/logdeck/internal/docker"
)

func main() {
	client, err := docker.NewClient()
	if err != nil {
		panic(err)
	}

	authService, err := auth.NewService()
	if err != nil {
		log.Fatalf("Failed to initialize auth service: %v\nPlease ensure ALL auth environment variables are set: JWT_SECRET, ADMIN_USERNAME, and ADMIN_PASSWORD.", err)
	}

	if authService == nil {
		log.Println("Authentication is DISABLED - no auth environment variables detected")
		log.Println("   To enable authentication, set: JWT_SECRET, ADMIN_USERNAME, ADMIN_PASSWORD")
	} else {
		log.Println("Authentication is ENABLED")
	}

	apiRouter := api.NewRouter(client, authService)

	log.Println("Server starting on :8080")
	http.ListenAndServe(":8080", apiRouter)
}
