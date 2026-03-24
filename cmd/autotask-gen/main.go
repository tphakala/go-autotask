package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

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
	entitiesFlag := flag.String("entities", "", "Comma-separated entity names to generate (default: built-in entity set)")
	flag.Parse()

	if *username == "" || *secret == "" || *integrationCode == "" {
		fmt.Fprintln(os.Stderr, "usage: autotask-gen -username USER -secret SECRET -integration-code CODE [-output DIR] [-entities NAMES]")
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
	if *entitiesFlag != "" {
		raw := strings.Split(*entitiesFlag, ",")
		entities := make([]string, 0, len(raw))
		for _, e := range raw {
			if name := strings.TrimSpace(e); name != "" {
				entities = append(entities, name)
			}
		}
		if len(entities) == 0 {
			return fmt.Errorf("no valid entity names provided via -entities")
		}
		gen.Entities = entities
	}
	if err := gen.Generate(ctx); err != nil {
		return fmt.Errorf("generation failed: %w", err)
	}
	fmt.Println("Generation complete.")
	return nil
}
