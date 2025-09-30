package main

import (
	"context"
	"log"
	"os"

	platform "github.com/synadia-io/platform-component"
)

func main() {

	token := os.Getenv("SCP_PLATFORM_TOKEN")
	scpUrl := os.Getenv("SCP_URL")

	pc := platform.Component("workloads", nil)
	err := pc.Register(
		token,
		platform.WithURL(scpUrl),
	)
	if err != nil {
		log.Fatalf("failed to register platform component: %v", err)
	}

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
