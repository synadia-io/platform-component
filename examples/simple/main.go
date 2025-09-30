package main

import (
	"context"
	"log"
	"os"

	platform "github.com/synadia-io/platform-component"
)

type Config struct {
	Account   string `json:"account"`
	NexusName string `json:"nexus_name"`
}

func main() {
	token := os.Getenv("SCP_PLATFORM_TOKEN")
	scpUrl := os.Getenv("SCP_URL")

	if token == "" {
		log.Fatalf("SCP_PLATFORM_TOKEN is not set")
	}
	if scpUrl == "" {
		scpUrl = platform.DefaultURL
	}

	cfg := &Config{}

	pc := platform.Component("workloads", nil)
	err := pc.Register(
		token,
		platform.WithURL(scpUrl),
		platform.WithConfig(cfg),
	)
	if err != nil {
		log.Fatalf("failed to register platform component: %v", err)
	}

	log.Println("registered platform component:")
	log.Printf("\taccount: %s", cfg.Account)
	log.Printf("\tnexus name: %s", cfg.NexusName)

	err = pc.Start(context.Background())
	if err != nil {
		log.Fatalf("failed to start platform component: %v", err)
	}
	defer pc.Stop()

	// closed by pc.Stop()
	nc := pc.NatsConnection()

	err = nc.Publish("test", []byte("test"))
	if err != nil {
		log.Fatalf("failed to publish message: %v", err)
	}

	log.Println("message published")
}
