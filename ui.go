package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	emoji "github.com/kyokomi/emoji/v2"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slack-go/slack"
)

var (
	timeStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	userStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	channelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	cursorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	dimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

type appState int

const (
	stateMessages appState = iota
	stateChannelSelect
	stateCompose
)

type slackMsgReceived struct{ text string }

type channelsLoadedMsg struct {
	channels []slack.Channel
	err      error
}

type model struct {
	state     appState
	messages  []string
	vp        viewport.Model
	input     textinput.Model
	channels  []slack.Channel
	chCursor  int
	chFilter  string
	chLoading bool
	selCh     *slack.Channel
	api       *slack.Client
	width     int
	height    int
	ready     bool
}

func newModel(api *slack.Client) model {
	ti := textinput.New()
	ti.CharLimit = 0

	return model{
		state: stateMessages,
		api:   api,
		input: ti,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if !m.ready {
			m.vp = viewport.New(msg.Width, msg.Height-1)
			m.ready = true
		} else {
			m.vp.Width = msg.Width
			m.vp.Height = msg.Height - 1
		}

	case slackMsgReceived:
		m.messages = append(m.messages, msg.text)
		m.vp.SetContent(strings.Join(m.messages, "\n"))
		m.vp.GotoBottom()

	case channelsLoadedMsg:
		m.chLoading = false
		if msg.err == nil {
			m.channels = msg.channels
		}

	case tea.KeyMsg:
		switch m.state {
		case stateMessages:
			switch msg.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "/":
				m.state = stateChannelSelect
				m.chCursor = 0
				m.chLoading = true
				m.channels = nil
				return m, loadChannels(m.api)
			default:
				var cmd tea.Cmd
				m.vp, cmd = m.vp.Update(msg)
				cmds = append(cmds, cmd)
			}

		case stateChannelSelect:
			filtered := m.filteredChannels()
			switch msg.String() {
			case "esc":
				m.state = stateMessages
				m.chFilter = ""
			case "up", "k":
				if m.chCursor > 0 {
					m.chCursor--
				}
			case "down", "j":
				if m.chCursor < len(filtered)-1 {
					m.chCursor++
				}
			case "enter":
				if len(filtered) > 0 {
					ch := filtered[m.chCursor]
					m.selCh = &ch
					m.input.SetValue("")
					m.input.Prompt = channelStyle.Render("#"+ch.Name) + " > "
					m.input.Focus()
					m.vp.Height = m.height - 2
					m.chFilter = ""
					m.state = stateCompose
					return m, textinput.Blink
				}
			case "backspace":
				if len(m.chFilter) > 0 {
					m.chFilter = m.chFilter[:len(m.chFilter)-1]
					m.chCursor = 0
				}
			default:
				if len(msg.Runes) == 1 {
					m.chFilter += string(msg.Runes)
					m.chCursor = 0
				}
			}

		case stateCompose:
			switch msg.String() {
			case "esc":
				m.state = stateMessages
				m.input.Blur()
				m.vp.Height = m.height - 1
			case "enter":
				text := m.input.Value()
				if text != "" && m.selCh != nil {
					ch := *m.selCh
					go m.api.PostMessage(ch.ID, slack.MsgOptionText(text, false))
					m.messages = append(m.messages, formatMsg(time.Now().Format("15:04:05"), "me", ch.Name, text))
					m.vp.SetContent(strings.Join(m.messages, "\n"))
					m.vp.GotoBottom()
				}
				m.state = stateMessages
				m.input.Blur()
				m.vp.Height = m.height - 1
			default:
				var cmd tea.Cmd
				m.input, cmd = m.input.Update(msg)
				cmds = append(cmds, cmd)
			}
		}
	}

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	if !m.ready {
		return "\n  Connecting to Slack..."
	}

	switch m.state {
	case stateMessages:
		return m.vp.View() + "\n" + dimStyle.Render(" press / to send a message")

	case stateChannelSelect:
		var sb strings.Builder
		sb.WriteString("\n  " + lipgloss.NewStyle().Bold(true).Render("Select channel") + "\n\n")
		filter := m.chFilter
		if filter == "" {
			filter = dimStyle.Render("type to filter...")
		} else {
			filter = channelStyle.Render(filter)
		}
		sb.WriteString("  / " + filter + "\n\n")
		if m.chLoading {
			sb.WriteString(dimStyle.Render("  loading..."))
		} else {
			filtered := m.filteredChannels()
			if len(filtered) == 0 {
				sb.WriteString(dimStyle.Render("  no channels found"))
			}
			for i, ch := range filtered {
				if i == m.chCursor {
					sb.WriteString("  " + cursorStyle.Render("> #"+ch.Name) + "\n")
				} else {
					sb.WriteString(dimStyle.Render("    #"+ch.Name) + "\n")
				}
			}
		}
		sb.WriteString("\n" + dimStyle.Render("  ↑/↓ or j/k · enter · esc"))
		return sb.String()

	case stateCompose:
		return m.vp.View() + "\n" + m.input.View()
	}

	return ""
}

func (m model) filteredChannels() []slack.Channel {
	if m.chFilter == "" {
		return m.channels
	}
	var result []slack.Channel
	for _, ch := range m.channels {
		if strings.Contains(strings.ToLower(ch.Name), strings.ToLower(m.chFilter)) {
			result = append(result, ch)
		}
	}
	return result
}

func formatMsg(ts, user, channel, text string) string {
	return fmt.Sprintf("%s %s %s: %s",
		timeStyle.Render(ts),
		userStyle.Render("@"+user),
		channelStyle.Render("(#"+channel+")"),
		emoji.Sprint(text),
	)
}

func loadChannels(api *slack.Client) tea.Cmd {
	return func() tea.Msg {
		var all []slack.Channel
		cursor := ""
		for {
			batch, next, err := api.GetConversations(&slack.GetConversationsParameters{
				ExcludeArchived: true,
				Types:           []string{"public_channel", "private_channel"},
				Cursor:          cursor,
			})
			if err != nil {
				return channelsLoadedMsg{err: err}
			}
			all = append(all, batch...)
			if next == "" {
				break
			}
			cursor = next
		}
		return channelsLoadedMsg{channels: all}
	}
}
