package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/AmoabaKelvin/logdeck/internal/api"
	"github.com/AmoabaKelvin/logdeck/internal/auth"
	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/AmoabaKelvin/logdeck/internal/docker"
	"github.com/AmoabaKelvin/logdeck/internal/system"
)

func main() {
	system.Init()

	config := config.NewConfig()
	fmt.Println("Config", config)

	multiHostClient, err := docker.NewMultiHostClient(config.DockerHosts)
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

	if config.ReadOnly {
		log.Println("READ-ONLY MODE is ENABLED - all mutating operations are disabled")
		log.Println("   To disable read-only mode, set: READONLY_MODE=false or unset the variable")
	} else {
		log.Println("Read-only mode is DISABLED - all operations are allowed")
	}

	apiRouter := api.NewRouter(multiHostClient, authService, config)

	log.Println("Server starting on :8080")
	if err := http.ListenAndServe(":8080", apiRouter); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
