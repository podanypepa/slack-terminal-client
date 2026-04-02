package main

import (
	"context"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

type listener struct {
	api          *slack.Client
	socket       *socketmode.Client
	userCache    map[string]string
	channelCache map[string]string
	mu           sync.RWMutex
	botName      string
	onMessage    func(ts, user, channel, text string)
}

func newListener(botToken, appToken, botName string, onMessage func(ts, user, channel, text string)) *listener {
	dialer := *websocket.DefaultDialer
	dialer.NetDialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		nd := &net.Dialer{}
		conn, err := nd.DialContext(ctx, network, addr)
		if err != nil {
			return nil, err
		}
		if tc, ok := conn.(*net.TCPConn); ok {
			_ = tc.SetKeepAlive(true)
			_ = tc.SetKeepAlivePeriod(5 * time.Second)
		}
		return conn, nil
	}
	api := slack.New(botToken, slack.OptionAppLevelToken(appToken))
	socket := socketmode.New(api, socketmode.OptionDialer(&dialer))
	return &listener{
		api:          api,
		socket:       socket,
		userCache:    make(map[string]string),
		channelCache: make(map[string]string),
		botName:      botName,
		onMessage:    onMessage,
	}
}

func (l *listener) run(ctx context.Context) error {
	go l.handleEvents(ctx)
	return l.socket.RunContext(ctx)
}

func (l *listener) handleEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-l.socket.Events:
			if !ok {
				return
			}
			if evt.Type != socketmode.EventTypeEventsAPI {
				if evt.Request != nil && evt.Request.EnvelopeID != "" {
					l.socket.Ack(*evt.Request)
				}
				continue
			}
			apiEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
			if !ok {
				if evt.Request != nil && evt.Request.EnvelopeID != "" {
					l.socket.Ack(*evt.Request)
				}
				continue
			}
			l.socket.Ack(*evt.Request)
			l.handleAPIEvent(apiEvent)
		}
	}
}

func (l *listener) handleAPIEvent(evt slackevents.EventsAPIEvent) {
	if evt.InnerEvent.Type != "message" {
		return
	}
	msg, ok := evt.InnerEvent.Data.(*slackevents.MessageEvent)
	if !ok || (msg.User == "" && msg.BotID == "") {
		return
	}

	ts := parseTimestamp(msg.TimeStamp)
	
	user := msg.User
	if msg.BotID != "" {
		user = l.botName
	} else {
		user = l.resolveUser(user)
	}

	channel := l.resolveChannel(msg.Channel)

	l.onMessage(ts.Format("15:04:05"), user, channel, msg.Text)
}

func (l *listener) resolveUser(id string) string {
	if id == "" {
		return "unknown"
	}
	l.mu.RLock()
	if name, ok := l.userCache[id]; ok {
		l.mu.RUnlock()
		return name
	}
	l.mu.RUnlock()

	user, err := l.api.GetUserInfo(id)

	l.mu.Lock()
	defer l.mu.Unlock()

	if err != nil {
		l.userCache[id] = id
		return id
	}
	name := user.Profile.DisplayName
	if name == "" {
		name = user.Name
	}
	l.userCache[id] = name
	return name
}

func (l *listener) resolveChannel(id string) string {
	l.mu.RLock()
	if name, ok := l.channelCache[id]; ok {
		l.mu.RUnlock()
		return name
	}
	l.mu.RUnlock()

	ch, err := l.api.GetConversationInfo(&slack.GetConversationInfoInput{ChannelID: id})

	l.mu.Lock()
	defer l.mu.Unlock()

	if err != nil {
		l.channelCache[id] = id
		return id
	}
	l.channelCache[id] = ch.Name
	return ch.Name
}

func parseTimestamp(ts string) time.Time {
	parts := strings.SplitN(ts, ".", 2)
	sec, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return time.Now()
	}
	return time.Unix(sec, 0)
}
