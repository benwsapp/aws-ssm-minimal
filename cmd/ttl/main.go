// Package main provides the CLI entrypoint for the TTL wrapper.
package main

import (
	"log"
	"os"

	"github.com/benwsapp/aws-ssm-minimal/internal/runner"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	app := runner.NewApp()

	code, err := app.Run()
	if err != nil {
		log.Printf("error: %v", err)
	}

	os.Exit(code)
}
