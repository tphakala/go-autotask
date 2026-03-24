package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	autotask "github.com/tphakala/go-autotask"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func run() error {
	username := flag.String("username", "", "Autotask API username")
	secret := flag.String("secret", "", "Autotask API secret")
	integrationCode := flag.String("integration-code", "", "Autotask API integration code")
	output := flag.String("output", "./entities", "Output directory for generated files")
	flag.Parse()

	if *username == "" || *secret == "" || *integrationCode == "" {
		fmt.Fprintln(os.Stderr, "usage: autotask-gen -username USER -secret SECRET -integration-code CODE [-output DIR]")
		os.Exit(1)
	}

	ctx := context.Background()
	auth := autotask.AuthConfig{
		Username:        *username,
		Secret:          *secret,
		IntegrationCode: *integrationCode,
	}

	client, err := autotask.NewClient(ctx, auth)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer func() { _ = client.Close() }()

	gen := &Generator{
		Client:    client,
		OutputDir: *output,
	}
	if err := gen.Generate(ctx); err != nil {
		return fmt.Errorf("generation failed: %w", err)
	}
	fmt.Println("Generation complete.")
	return nil
}
