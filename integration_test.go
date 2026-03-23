//go:build integration

package autotask_test

import (
	"context"
	"os"
	"testing"

	autotask "github.com/tphakala/go-autotask"
	"github.com/tphakala/go-autotask/entities"
	"github.com/tphakala/go-autotask/metadata"
)

func integrationClient(t *testing.T) *autotask.Client {
	t.Helper()
	username := os.Getenv("AUTOTASK_USERNAME")
	secret := os.Getenv("AUTOTASK_SECRET")
	code := os.Getenv("AUTOTASK_INTEGRATION_CODE")
	if username == "" || secret == "" || code == "" {
		t.Skip("AUTOTASK_USERNAME, AUTOTASK_SECRET, AUTOTASK_INTEGRATION_CODE required")
	}
	client, err := autotask.NewClient(context.Background(), autotask.AuthConfig{
		Username: username, Secret: secret, IntegrationCode: code,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() })
	return client
}

func TestIntegrationZoneDiscovery(t *testing.T) {
	client := integrationClient(t)
	// Client was created successfully, which means zone discovery worked.
	// We can't access client.baseURL (unexported) from an external test package,
	// but successful client creation proves zone discovery completed.
	_ = client
	t.Log("Zone discovery succeeded (client created without error)")
}

func TestIntegrationListTickets(t *testing.T) {
	client := integrationClient(t)
	tickets, err := autotask.List[entities.Ticket](context.Background(), client,
		autotask.NewQuery().Where("status", autotask.OpEq, 1).Limit(5),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Found %d open tickets", len(tickets))
}

func TestIntegrationGetFields(t *testing.T) {
	client := integrationClient(t)
	fields, err := metadata.GetFields(context.Background(), client, "Tickets")
	if err != nil {
		t.Fatal(err)
	}
	if len(fields) == 0 {
		t.Fatal("expected at least one field")
	}
	t.Logf("Found %d fields for Tickets", len(fields))
}

func TestIntegrationCountTickets(t *testing.T) {
	client := integrationClient(t)
	count, err := autotask.Count[entities.Ticket](context.Background(), client, autotask.NewQuery())
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Total tickets: %d", count)
}
