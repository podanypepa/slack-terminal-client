package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	emoji "github.com/kyokomi/emoji/v2"
	"github.com/slack-go/slack"
)

var (
	timeStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	userStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	channelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	cursorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	dimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	mentionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
)

type appState int

const (
	stateMessages appState = iota
	stateChannelSelect
	stateCompose
	stateMention
)

// mentionPickerLines is the fixed height of the mention picker:
// 1 filter line + 5 result lines + 1 hint line + 1 separator = 8
const mentionPickerLines = 8

type rawMsg struct {
	ts      string
	user    string
	channel string
	text    string
}

type slackMsgReceived struct {
	ts      string
	user    string
	channel string
	text    string
}

type historyLoadedMsg struct {
	messages []rawMsg
}

type channelsLoadedMsg struct {
	channels []slack.Channel
	err      error
}

type usersLoadedMsg struct {
	users []slack.User
	err   error
}

func loadHistory(api *slack.Client, botName string) tea.Cmd {
	return func() tea.Msg {
		channels, _, err := api.GetConversations(&slack.GetConversationsParameters{
			ExcludeArchived: true,
			Types:           []string{"public_channel", "private_channel"},
		})
		if err != nil {
			return historyLoadedMsg{messages: []rawMsg{{text: dimStyle.Render("!! Error loading history channels: " + err.Error())}}}
		}

		userCache := make(map[string]string)
		getUserName := func(id string) string {
			if name, ok := userCache[id]; ok {
				return name
			}
			user, err := api.GetUserInfo(id)
			if err != nil {
				return id
			}
			name := user.Profile.DisplayName
			if name == "" {
				name = user.Name
			}
			userCache[id] = name
			return name
		}

		type msgRecord struct {
			sortTS string
			msg    rawMsg
		}
		var allMsgs []msgRecord

		for _, ch := range channels {
			if !ch.IsMember {
				continue
			}
			history, err := api.GetConversationHistory(&slack.GetConversationHistoryParameters{
				ChannelID: ch.ID,
				Limit:     10,
			})
			if err != nil {
				continue
			}

			for _, m := range history.Messages {
				if m.User == "" && m.BotID == "" {
					continue
				}

				ts := parseTimestamp(m.Timestamp)

				var userName string
				if m.BotID != "" {
					userName = botName
				} else {
					userName = getUserName(m.User)
				}

				allMsgs = append(allMsgs, msgRecord{
					sortTS: m.Timestamp,
					msg: rawMsg{
						ts:      ts.Format("15:04:05"),
						user:    userName,
						channel: ch.Name,
						text:    resolveMentions(m.Text, getUserName),
					},
				})
			}
		}

		sort.Slice(allMsgs, func(i, j int) bool {
			return allMsgs[i].sortTS < allMsgs[j].sortTS
		})

		var result []rawMsg
		for _, m := range allMsgs {
			result = append(result, m.msg)
		}

		return historyLoadedMsg{messages: result}
	}
}

func loadUsers(api *slack.Client) tea.Cmd {
	return func() tea.Msg {
		users, err := api.GetUsers()
		return usersLoadedMsg{users: users, err: err}
	}
}

type model struct {
	state          appState
	messages       []rawMsg
	vp             viewport.Model
	input          textinput.Model
	channels       []slack.Channel
	chCursor       int
	chFilter       string
	chLoading      bool
	selCh          *slack.Channel
	mentionUsers   []slack.User
	mentionCursor  int
	mentionFilter  string
	mentionLoading bool
	api            *slack.Client
	botName        string
	width          int
	height         int
	ready          bool
}

func newModel(api *slack.Client, botName string) model {
	ti := textinput.New()
	ti.CharLimit = 0

	return model{
		state:   stateMessages,
		api:     api,
		input:   ti,
		botName: botName,
	}
}

func (m model) Init() tea.Cmd {
	return loadHistory(m.api, m.botName)
}

func (m model) viewportContent() string {
	lines := make([]string, len(m.messages))
	for i, msg := range m.messages {
		lines[i] = formatMsg(msg.ts, msg.user, msg.channel, msg.text, m.width)
	}
	return strings.Join(lines, "\n")
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
		m.vp.SetContent(m.viewportContent())

	case slackMsgReceived:
		m.messages = append(m.messages, rawMsg(msg))
		if len(m.messages) > 500 {
			m.messages = m.messages[len(m.messages)-500:]
		}
		m.vp.SetContent(m.viewportContent())
		m.vp.GotoBottom()

	case historyLoadedMsg:
		m.messages = append(m.messages, msg.messages...)
		if len(m.messages) > 500 {
			m.messages = m.messages[len(m.messages)-500:]
		}
		m.vp.SetContent(m.viewportContent())
		m.vp.GotoBottom()

	case channelsLoadedMsg:
		m.chLoading = false
		if msg.err == nil {
			m.channels = msg.channels
		}

	case usersLoadedMsg:
		m.mentionLoading = false
		if msg.err == nil {
			m.mentionUsers = msg.users
		}

	case messageSentMsg:
		if msg.err != nil {
			m.messages = append(m.messages, rawMsg{text: dimStyle.Render(fmt.Sprintf("!! Error sending message: %v", msg.err))})
			if len(m.messages) > 500 {
				m.messages = m.messages[len(m.messages)-500:]
			}
			m.vp.SetContent(m.viewportContent())
			m.vp.GotoBottom()
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
					m.selCh = &filtered[m.chCursor]
					m.input.SetValue("")
					m.input.Prompt = channelStyle.Render("#"+m.selCh.Name) + " > "
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
				var cmd tea.Cmd
				if text != "" && m.selCh != nil {
					cmd = sendMessage(m.api, m.selCh.ID, m.selCh.Name, text)
				}
				m.state = stateMessages
				m.input.Blur()
				m.vp.Height = m.height - 1
				return m, cmd
			case "@":
				// Trigger mention picker only when @ follows a space or at start.
				val := m.input.Value()
				if val == "" || strings.HasSuffix(val, " ") {
					m.state = stateMention
					m.mentionCursor = 0
					m.mentionFilter = ""
					m.vp.Height = max(1, m.height-2-mentionPickerLines)
					if len(m.mentionUsers) == 0 {
						m.mentionLoading = true
						cmds = append(cmds, loadUsers(m.api))
					}
				} else {
					var cmd tea.Cmd
					m.input, cmd = m.input.Update(msg)
					cmds = append(cmds, cmd)
				}
			default:
				var cmd tea.Cmd
				m.input, cmd = m.input.Update(msg)
				cmds = append(cmds, cmd)
			}

		case stateMention:
			filtered := m.filteredUsers()
			switch msg.String() {
			case "esc":
				m.state = stateCompose
				m.mentionFilter = ""
				m.vp.Height = m.height - 2
			case "up", "k":
				if m.mentionCursor > 0 {
					m.mentionCursor--
				}
			case "down", "j":
				if m.mentionCursor < len(filtered)-1 && m.mentionCursor < 4 {
					m.mentionCursor++
				}
			case "enter":
				if len(filtered) > 0 {
					u := filtered[m.mentionCursor]
					m.input.SetValue(m.input.Value() + "<@" + u.ID + "> ")
				}
				m.state = stateCompose
				m.mentionFilter = ""
				m.vp.Height = m.height - 2
			case "backspace":
				if len(m.mentionFilter) > 0 {
					runes := []rune(m.mentionFilter)
					m.mentionFilter = string(runes[:len(runes)-1])
					m.mentionCursor = 0
				}
			default:
				if len(msg.Runes) == 1 {
					m.mentionFilter += string(msg.Runes)
					m.mentionCursor = 0
				}
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

	case stateMention:
		var sb strings.Builder
		sb.WriteString(m.vp.View())
		sb.WriteString("\n")

		// Filter line
		filter := m.mentionFilter
		if filter == "" {
			filter = dimStyle.Render("type to filter...")
		} else {
			filter = mentionStyle.Render(filter)
		}
		sb.WriteString("  @ " + filter + "\n")

		// Result lines (always 5 slots to keep layout stable)
		if m.mentionLoading {
			sb.WriteString(dimStyle.Render("  loading users...") + "\n")
			for i := 1; i < 5; i++ {
				sb.WriteString("\n")
			}
		} else {
			filtered := m.filteredUsers()
			for i := range 5 {
				if i >= len(filtered) {
					sb.WriteString("\n")
					continue
				}
				name := userDisplayName(filtered[i])
				if i == m.mentionCursor {
					sb.WriteString("  " + cursorStyle.Render("> @"+name) + "\n")
				} else {
					sb.WriteString(dimStyle.Render("    @"+name) + "\n")
				}
			}
		}

		sb.WriteString(dimStyle.Render("  ↑/↓ or j/k · enter · esc") + "\n")
		sb.WriteString(m.input.View())
		return sb.String()
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

func (m model) filteredUsers() []slack.User {
	var result []slack.User
	for _, u := range m.mentionUsers {
		if u.Deleted || u.IsBot || u.ID == "USLACKBOT" {
			continue
		}
		name := userDisplayName(u)
		if m.mentionFilter == "" || strings.Contains(strings.ToLower(name), strings.ToLower(m.mentionFilter)) {
			result = append(result, u)
		}
	}
	return result
}

func userDisplayName(u slack.User) string {
	if u.Profile.DisplayName != "" {
		return u.Profile.DisplayName
	}
	return u.Name
}

var mentionReUI = regexp.MustCompile(`<@([A-Z0-9]+)>`)

func resolveMentions(text string, getName func(string) string) string {
	return mentionReUI.ReplaceAllStringFunc(text, func(match string) string {
		id := match[2 : len(match)-1]
		return mentionStyle.Render("@" + getName(id))
	})
}

// wrapLine splits a plain-text line into lines of at most width visual cells.
func wrapLine(s string, width int) []string {
	if width <= 0 || lipgloss.Width(s) <= width {
		return []string{s}
	}
	words := strings.Fields(s)
	if len(words) == 0 {
		return []string{s}
	}
	var lines []string
	current := ""
	currentWidth := 0
	for _, word := range words {
		ww := lipgloss.Width(word)
		switch {
		case currentWidth == 0:
			current = word
			currentWidth = ww
		case currentWidth+1+ww <= width:
			current += " " + word
			currentWidth += 1 + ww
		default:
			lines = append(lines, current)
			current = word
			currentWidth = ww
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

func formatMsg(ts, user, channel, text string, width int) string {
	// Pre-formatted messages (e.g. error strings) have no ts/user/channel.
	if ts == "" {
		return text
	}

	prefix := fmt.Sprintf("%s %s %s: ",
		timeStyle.Render(ts),
		userStyle.Render("@"+user),
		channelStyle.Render("(#"+channel+")"),
	)

	indentWidth := lipgloss.Width(prefix)
	indent := strings.Repeat(" ", indentWidth)
	contentWidth := width - indentWidth

	content := emoji.Sprint(text)
	textLines := strings.Split(content, "\n")

	var resultLines []string
	for i, line := range textLines {
		var wrapped []string
		if contentWidth > 10 {
			wrapped = wrapLine(line, contentWidth)
		} else {
			wrapped = []string{line}
		}
		if len(wrapped) == 0 {
			wrapped = []string{""}
		}
		for j, wl := range wrapped {
			if i == 0 && j == 0 {
				resultLines = append(resultLines, prefix+wl)
			} else {
				resultLines = append(resultLines, indent+wl)
			}
		}
	}

	return strings.Join(resultLines, "\n")
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

type messageSentMsg struct {
	text    string
	channel string
	err     error
}

func sendMessage(api *slack.Client, channelID, channelName, text string) tea.Cmd {
	return func() tea.Msg {
		_, _, err := api.PostMessage(channelID, slack.MsgOptionText(text, false))
		return messageSentMsg{
			text:    text,
			channel: channelName,
			err:     err,
		}
	}
}
