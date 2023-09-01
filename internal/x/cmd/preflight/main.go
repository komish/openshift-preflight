package main

import (
	"context"
	"log"

	"github.com/redhat-openshift-ecosystem/openshift-preflight/internal/x/cmd/preflight/root"
)

func main() {
	entrypoint := root.NewCommand(
		context.Background(),
	)

	if err := entrypoint.Execute(); err != nil {
		log.Fatal(err)
	}
}
