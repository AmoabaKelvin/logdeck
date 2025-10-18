package main

import (
	"net/http"

	"github.com/AmoabaKelvin/logdeck/internal/api"
	"github.com/AmoabaKelvin/logdeck/internal/docker"
)

func main() {
	client, err := docker.NewClient()
	if err != nil {
		panic(err)
	}

	apiRouter := api.NewRouter(client)

	http.ListenAndServe(":8080", apiRouter)
}
