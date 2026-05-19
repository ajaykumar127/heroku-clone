package main

import (
	"context"
	"log"
	"net/http"
	"time"
)

func main() {
	cfg := LoadConfig()
	log.Printf("Runtime Agent starting: name=%s cloud=%s region=%s",
		cfg.RuntimeName, cfg.Cloud, cfg.Region)

	agent := &Agent{
		config: cfg,
		client: &http.Client{Timeout: 30 * time.Second},
	}

	if err := agent.Register(); err != nil {
		log.Fatalf("Failed to register with control plane: %v", err)
	}
	log.Printf("Registered as runtime ID: %s", agent.runtimeID)

	go agent.sendHeartbeat()

	ctx := context.Background()
	agent.Run(ctx)
}
