// Package main is the entry point of the application. It contains the main function which is the starting point of the program execution.
package main

import (
	"os"

	"github.com/caarlos0/env/v11"
	"github.com/fatih/color"
	"github.com/joho/godotenv"
	"github.com/spf13/pflag"
)

func main() {
	var c string // config file path

	pflag.StringVarP(&c, "config", "c", ".env", "Path to .env file")
	pflag.Parse()

	if err := godotenv.Load(c); err != nil {
		color.Red("No .env file found at %s", c)
		os.Exit(1)
	}

	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		color.Red("Failed to parse env: %v", err)
		return
	}
}
