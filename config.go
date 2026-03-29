package main

// Config is a struct that holds the configuration values for the application. It is populated from environment variables using the env package.
type Config struct {
	SlackBotToken string `env:"SLACK_BOT_TOKEN,required"`
	SlackAppToken string `env:"SLACK_APP_TOKEN,required"`
	BotName       string `env:"BOT_NAME" envDefault:"BOT"`
}
