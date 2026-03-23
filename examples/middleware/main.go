package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"

	autotask "github.com/tphakala/go-autotask"
	"github.com/tphakala/go-autotask/entities"
	"github.com/tphakala/go-autotask/middleware"
)

func main() {
	ctx := context.Background()
	client, err := autotask.NewClient(ctx,
		autotask.AuthConfig{
			Username:        os.Getenv("AUTOTASK_USERNAME"),
			Secret:          os.Getenv("AUTOTASK_SECRET"),
			IntegrationCode: os.Getenv("AUTOTASK_INTEGRATION_CODE"),
		},
		autotask.WithLogger(slog.Default()),
		autotask.WithRateLimiter(
			middleware.WithRequestsPerHour(8000),
			middleware.WithBurstSize(10),
		),
		autotask.WithCircuitBreaker(
			middleware.WithFailureThreshold(5),
		),
		autotask.WithThresholdMonitor(
			middleware.WithCriticalCallback(func(info middleware.ThresholdInfo) {
				slog.Error("API usage critical",
					"percent", fmt.Sprintf("%.1f%%", info.UsagePercent),
					"current", info.CurrentUsage,
					"threshold", info.Threshold,
				)
			}),
		),
		autotask.WithImpersonation(12345),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	ticket, err := autotask.Get[entities.Ticket](ctx, client, 1)
	if err != nil {
		log.Println("Failed to get ticket:", err)
		return
	}
	if title, ok := ticket.Title.Get(); ok {
		fmt.Printf("Ticket: %s\n", title)
	} else {
		fmt.Println("Ticket title not set")
	}
}
