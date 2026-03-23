package main

import (
	"context"
	"fmt"
	"log"
	"os"

	autotask "github.com/tphakala/go-autotask"
	"github.com/tphakala/go-autotask/entities"
)

func main() {
	ctx := context.Background()
	username := os.Getenv("AUTOTASK_USERNAME")
	secret := os.Getenv("AUTOTASK_SECRET")
	integrationCode := os.Getenv("AUTOTASK_INTEGRATION_CODE")
	if username == "" || secret == "" || integrationCode == "" {
		log.Fatal("AUTOTASK_USERNAME, AUTOTASK_SECRET, and AUTOTASK_INTEGRATION_CODE must be set")
	}
	client, err := autotask.NewClient(ctx, autotask.AuthConfig{
		Username:        username,
		Secret:          secret,
		IntegrationCode: integrationCode,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	// Query open high-priority tickets.
	tickets, err := autotask.List[entities.Ticket](ctx, client,
		autotask.NewQuery().
			Where("status", autotask.OpEq, 1).
			Or(
				autotask.Field("priority", autotask.OpEq, 1),
				autotask.Field("priority", autotask.OpEq, 2),
			).
			Fields("id", "title", "status", "priority").
			Limit(50),
	)
	if err != nil {
		log.Fatal(err)
	}
	for _, t := range tickets {
		id, _ := t.ID.Get()
		title, _ := t.Title.Get()
		fmt.Printf("  [%d] %s\n", id, title)
	}

	// Iterator-based pagination for large result sets.
	fmt.Println("\nAll tickets (iterator):")
	for ticket, err := range autotask.ListIter[entities.Ticket](ctx, client, autotask.NewQuery()) {
		if err != nil {
			log.Fatal(err)
		}
		title, _ := ticket.Title.Get()
		fmt.Printf("  %s\n", title)
	}
}
