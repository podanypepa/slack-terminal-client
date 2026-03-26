# slack-terminal-client
monitoring slack activities from terminal window

## Slack App Setup

### 1. Create a Slack App

Go to [api.slack.com/apps](https://api.slack.com/apps) → **Create New App** → **From scratch**.

### 2. Enable Socket Mode

**Settings → Socket Mode** → enable → generate an **App-Level Token** (use as `SLACK_APP_TOKEN`, prefix `xapp-`).

### 3. Add Bot Token Scopes

**OAuth & Permissions → Bot Token Scopes** add:

| Scope | Description |
|---|---|
| `channels:history` | read messages in public channels |
| `groups:history` | read messages in private channels |
| `im:history` | read direct messages |
| `channels:read` | resolve channel names |
| `users:read` | resolve user names |

### 4. Subscribe to Events

**Event Subscriptions** → enable → **Subscribe to bot events** add:

- `message.channels`
- `message.groups` (optional, for private channels)

### 5. Install the App to Your Workspace

**OAuth & Permissions → Install to Workspace** → copy the **Bot User OAuth Token** (use as `SLACK_BOT_TOKEN`, prefix `xoxb-`).

## Configuration

Create a `.env` file in the project root:

```env
SLACK_BOT_TOKEN=xoxb-...
SLACK_APP_TOKEN=xapp-...
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

Output:

```
Connecting to Slack...
15:04:05 @john (#general): Hey everyone!
15:04:42 @pepa (#dev): merge done
```
