package main

import (
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"toxiproxy-poc/internal/app"

	"github.com/nats-io/nats.go"
)

const (
	envNatsUrl  = "NATS_URL"
	envPort     = "PORT"
	defaultPort = 8081
)

func main() {
	natsUrl := os.Getenv(envNatsUrl)
	if natsUrl == "" {
		natsUrl = nats.DefaultURL
	}

	portStr := os.Getenv(envPort)
	port := defaultPort

	if portStr != "" {
		p, err := strconv.Atoi(portStr)
		if err != nil {
			log.Fatalf("can't set port: %v\n", err)
		}
		port = p
	}

	cfg := app.NewConfig(natsUrl, port)

	svc, err := app.NewService(cfg)
	if err != nil {
		log.Fatalf("can't create service: error = %v\n", err)
	}

	svc.Start()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs

	svc.Stop()
}
