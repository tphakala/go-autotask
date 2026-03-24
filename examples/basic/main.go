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
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx := context.Background()
	client, err := autotask.NewClient(ctx, autotask.AuthConfig{
		Username:        os.Getenv("AUTOTASK_USERNAME"),
		Secret:          os.Getenv("AUTOTASK_SECRET"),
		IntegrationCode: os.Getenv("AUTOTASK_INTEGRATION_CODE"),
	})
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	// Get a ticket by ID.
	ticket, err := autotask.Get[entities.Ticket](ctx, client, 12345) //nolint:mnd // placeholder ticket ID for example
	if err != nil {
		return err
	}
	if title, ok := ticket.Title.Get(); ok {
		fmt.Printf("Ticket: %s\n", title)
	}

	// Create a ticket.
	newTicket := &entities.Ticket{
		Title:     autotask.Set("Server unreachable"),
		CompanyID: autotask.Set(int64(0)), // TODO: Replace with a valid company ID
		Status:    autotask.Set(1),
		Priority:  autotask.Set(2), //nolint:mnd // ticket priority value for example
	}
	created, err := autotask.Create(ctx, client, newTicket)
	if err != nil {
		return err
	}
	fmt.Printf("Created ticket: %v\n", created)
	return nil
}
