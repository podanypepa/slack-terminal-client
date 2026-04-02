# slack-terminal-client

> All your Slack messages. One terminal. No switching.

## Motivation

I live in the terminal. As a developer, DevOps engineer and sysadmin who's constantly SSH'd into something, my terminal is home — and I have no intention of leaving it just because someone pinged me on Slack.

The problem with Slack is that messages are scattered across dozens of channels. Finding them means switching away from the terminal, hunting through the sidebar, reading, switching back. Every. Single. Time.

So I built this. `slack-terminal-client` collects all incoming Slack messages and displays them in one clean, chronological stream — who wrote it, when, and in which channel. And when I need to reply, I do it right there, without ever touching the Slack app. No alt-tab, no context switch, no ding from a notification I'm going to ignore anyway.

Just me and my terminal, the way it should be.

## Slack App Setup

### 1. Create a Slack App

Go to [api.slack.com/apps](https://api.slack.com/apps) → **Create New App** → **From scratch**.

### 2. Enable Socket Mode

**Settings → Socket Mode** → enable → generate an **App-Level Token** (use as `SLACK_APP_TOKEN`, prefix `xapp-`).

When generating the token, add the `connections:write` scope.

### 3. Add Bot Token Scopes

**OAuth & Permissions → Bot Token Scopes** add:

| Scope | Description |
|---|---|
| `channels:history` | read messages in public channels |
| `groups:history` | read messages in private channels |
| `im:history` | read direct messages |
| `channels:read` | list and resolve public channel names |
| `groups:read` | list and resolve private channel names |
| `users:read` | resolve user names |
| `chat:write` | send messages as the bot |

### 4. Subscribe to Events

**Event Subscriptions** → enable → **Subscribe to bot events** add:

- `message.channels`
- `message.groups`

Save changes.

### 5. Install the App to Your Workspace

**OAuth & Permissions → Install to Workspace** → copy the **Bot User OAuth Token** (use as `SLACK_BOT_TOKEN`, prefix `xoxb-`).

### 6. Add the Bot to Channels

In Slack, open any channel you want to monitor and run:

```
/invite @your-bot-name
```

## Configuration

Create a `.env` file in the project root:

```env
SLACK_BOT_TOKEN=xoxb-...
SLACK_APP_TOKEN=xapp-...

# Optional: customize your bot's display name (text or emoji). Default is "BOT".
BOT_NAME=🤖

# Optional: color for @mentions in message text. Accepts ANSI 256-color codes or hex.
# Default is "3" (yellow). Examples: "220" (bright yellow), "#ffcc00" (hex).
MENTION_COLOR=3
```

## Running

```bash
go build -o slack-terminal-client .
./slack-terminal-client
```

Or with a custom config path:

```bash
./slack-terminal-client -c /path/to/.env
```

## Usage

| Key | Action |
|---|---|
| `/` | open channel selector |
| `↑` / `↓` or `j` / `k` | navigate channels |
| type letters | filter channels by name |
| `Backspace` | clear filter character |
| `Enter` | select channel / send message |
| `Esc` | cancel / go back |
| `Ctrl+C` | quit |

Messages are displayed as:

```
15:04:05 @john (#general): Hey everyone!
15:04:42 @pepa (#dev): merge done
```
