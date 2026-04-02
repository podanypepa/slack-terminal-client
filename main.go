// Package main implements a terminal-based Slack client that listens for messages in real-time and displays them in a user-friendly interface.
// It uses the Bubble Tea framework for the UI and the caarlos0/env package for configuration management.
// The application loads environment variables from a specified .env file, sets up signal handling for graceful shutdown, and
// initializes a Slack listener to receive messages. When a message is received, it sends the message details to the UI for display.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/caarlos0/env/v11"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/joho/godotenv"
	"github.com/spf13/pflag"
)

func main() {
	if err := run(); err != nil {
		os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
}

func run() error {
	var c string
	pflag.StringVarP(&c, "config", "c", ".env", "Path to .env file")
	pflag.Parse()

	if err := godotenv.Load(c); err != nil {
		return fmt.Errorf("no .env file found at %s", c)
	}

	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return fmt.Errorf("failed to parse env: %w", err)
	}

	mentionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.MentionColor))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		cancel()
	}()

	l := newListener(cfg.SlackBotToken, cfg.SlackAppToken, cfg.BotName, nil)
	p := tea.NewProgram(newModel(l.api, cfg.BotName), tea.WithAltScreen())
	l.onMessage = func(ts, user, channel, text string) {
		p.Send(slackMsgReceived{ts: ts, user: user, channel: channel, text: text})
	}

	go func() {
		if err := l.run(ctx); err != nil && err != context.Canceled {
			cancel()
		}
	}()

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("UI error: %w", err)
	}
	return nil
}
