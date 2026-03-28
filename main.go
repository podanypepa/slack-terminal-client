package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/caarlos0/env/v11"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/joho/godotenv"
	"github.com/spf13/pflag"
)

func main() {
	var c string
	pflag.StringVarP(&c, "config", "c", ".env", "Path to .env file")
	pflag.Parse()

	if err := godotenv.Load(c); err != nil {
		os.Stderr.WriteString("No .env file found at " + c + "\n")
		os.Exit(1)
	}

	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		os.Stderr.WriteString("Failed to parse env\n")
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		cancel()
	}()

	l := newListener(cfg.SlackBotToken, cfg.SlackAppToken, nil)
	p := tea.NewProgram(newModel(l.api), tea.WithAltScreen())
	l.onMessage = func(text string) { p.Send(slackMsgReceived{text: text}) }

	go func() {
		if err := l.run(ctx); err != nil && err != context.Canceled {
			cancel()
		}
	}()

	if _, err := p.Run(); err != nil {
		os.Stderr.WriteString("UI error: " + err.Error() + "\n")
		os.Exit(1)
	}
}
