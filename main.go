// Package main is the entry point of the application. It contains the main function which is the starting point of the program execution.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		cancel()
	}()

	color.Green("Connecting to Slack...")
	l := newListener(cfg.SlackBotToken, cfg.SlackAppToken)
	if err := l.run(ctx); err != nil && err != context.Canceled {
		color.Red("Error: %v", err)
		os.Exit(1)
	}
}
