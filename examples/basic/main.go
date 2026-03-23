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
	client, err := autotask.NewClient(ctx, autotask.AuthConfig{
		Username:        os.Getenv("AUTOTASK_USERNAME"),
		Secret:          os.Getenv("AUTOTASK_SECRET"),
		IntegrationCode: os.Getenv("AUTOTASK_INTEGRATION_CODE"),
	})
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	// Get a ticket by ID.
	ticket, err := autotask.Get[entities.Ticket](ctx, client, 12345)
	if err != nil {
		log.Fatal(err)
	}
	if title, ok := ticket.Title.Get(); ok {
		fmt.Printf("Ticket: %s\n", title)
	}

	// Create a ticket.
	newTicket := &entities.Ticket{
		Title:     autotask.Set("Server unreachable"),
		CompanyID: autotask.Set(int64(0)),
		Status:    autotask.Set(1),
		Priority:  autotask.Set(2),
	}
	created, err := autotask.Create(ctx, client, newTicket)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Created ticket: %v\n", created)
}
