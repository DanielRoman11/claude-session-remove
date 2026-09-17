package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Claude Code's own palette: a warm terracotta accent over neutral
// stone grays, adapted for both light and dark terminals.
var (
	colorAccent = lipgloss.AdaptiveColor{Light: "#C2410C", Dark: "#DA7756"}
	colorText   = lipgloss.AdaptiveColor{Light: "#292524", Dark: "#E7E5E4"}
	colorMuted  = lipgloss.AdaptiveColor{Light: "#78716C", Dark: "#A8A29E"}
	colorFaint  = lipgloss.AdaptiveColor{Light: "#D6D3D1", Dark: "#57534E"}
	colorDanger = lipgloss.AdaptiveColor{Light: "#B91C1C", Dark: "#E5484D"}
	colorOnAcc  = lipgloss.AdaptiveColor{Light: "#FFFBEB", Dark: "#1C1917"}

	appTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
	subtitleStyle = lipgloss.NewStyle().Foreground(colorMuted)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorAccent).
			Padding(1, 2)

	cursorStyle = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)

	itemStyle = lipgloss.NewStyle().Foreground(colorText)

	selectedItemStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorOnAcc).
				Background(colorAccent)

	currentTagStyle = lipgloss.NewStyle().Foreground(colorAccent).Italic(true)
	timeStyle       = lipgloss.NewStyle().Foreground(colorFaint)
	dimStyle        = lipgloss.NewStyle().Foreground(colorMuted)

	helpKeyStyle  = lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	helpDescStyle = lipgloss.NewStyle().Foreground(colorMuted)

	dialogTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(colorDanger)

	buttonStyle = lipgloss.NewStyle().Padding(0, 3).MarginRight(2).
			Border(lipgloss.RoundedBorder()).BorderForeground(colorFaint).Foreground(colorMuted)

	buttonDangerStyle = lipgloss.NewStyle().Padding(0, 3).MarginRight(2).Bold(true).
				Foreground(colorOnAcc).Background(colorDanger)

	successStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#15803D", Dark: "#4ADE80"})
)

type screen int

const (
	screenList screen = iota
	screenConfirm
)

type model struct {
	sessions  []session
	cursor    int
	screen    screen
	currentID string
	target    session
	confirmed bool
	width     int
	height    int
}

func newModel(sessions []session, currentID string) model {
	return model{sessions: sessions, currentID: currentID}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case tea.KeyMsg:
		switch m.screen {
		case screenList:
			switch msg.String() {
			case "ctrl+c", "q", "esc":
				return m, tea.Quit
			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
				}
			case "down", "j":
				if m.cursor < len(m.sessions)-1 {
					m.cursor++
				}
			case "enter", "d":
				if len(m.sessions) > 0 {
					m.target = m.sessions[m.cursor]
					m.screen = screenConfirm
				}
			}
		case screenConfirm:
			switch msg.String() {
			case "y", "enter":
				m.confirmed = true
				return m, tea.Quit
			case "n", "esc", "ctrl+c":
				m.screen = screenList
			}
		}
	}
	return m, nil
}

func (m model) View() string {
	switch m.screen {
	case screenConfirm:
		return m.viewConfirm()
	default:
		return m.viewList()
	}
}

// listInnerWidth is the fixed width of a row's text (title + time), so the
// time column lands flush right and stays put as the cursor moves between
// rows of different title lengths.
const listInnerWidth = 54

// timeColWidth reserves room for the widest relTime() output ("59m ago",
// "23h ago", "29d ago") plus a one-space gap before it.
const timeColWidth = 8

func truncateEllipsis(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 1 {
		return "…"
	}
	return string(r[:max-1]) + "…"
}

func (m model) viewList() string {
	header := appTitleStyle.Render("Claude Code Sessions") + "  " +
		subtitleStyle.Render(fmt.Sprintf("%d found", len(m.sessions)))

	var rows []string
	for i, s := range m.sessions {
		tag := ""
		if m.currentID != "" && s.ID == m.currentID {
			tag = currentTagStyle.Render("  ● current")
		}

		titleMax := listInnerWidth - timeColWidth - lipgloss.Width(tag)
		if titleMax < 4 {
			titleMax = 4
		}
		left := truncateEllipsis(s.Title, titleMax) + tag

		timeStr := relTime(s.ModTime)
		pad := listInnerWidth - lipgloss.Width(left) - lipgloss.Width(timeStr)
		if pad < 1 {
			pad = 1
		}
		line := left + strings.Repeat(" ", pad) + timeStyle.Render(timeStr)

		cursor := "  "
		if i == m.cursor {
			cursor = cursorStyle.Render("› ")
			rows = append(rows, cursor+selectedItemStyle.Render(" "+line+" "))
		} else {
			rows = append(rows, cursor+itemStyle.Render(line))
		}
	}
	list := strings.Join(rows, "\n")

	help := helpKeyStyle.Render("↑/↓") + " " + helpDescStyle.Render("navigate") + "    " +
		helpKeyStyle.Render("enter/d") + " " + helpDescStyle.Render("delete") + "    " +
		helpKeyStyle.Render("q") + " " + helpDescStyle.Render("quit")

	body := lipgloss.JoinVertical(lipgloss.Left, header, "", list, "", help)
	box := boxStyle.Render(body)

	if m.width == 0 {
		return box
	}
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m model) viewConfirm() string {
	title := dialogTitleStyle.Render("Delete this session?")
	name := itemStyle.Render("\"" + m.target.Title + "\"")
	path := timeStyle.Render(m.target.Path)

	buttons := lipgloss.JoinHorizontal(lipgloss.Top,
		buttonDangerStyle.Render("y Delete"),
		buttonStyle.Render("n Cancel"),
	)

	body := lipgloss.JoinVertical(lipgloss.Left, title, "", name, path, "", buttons)
	box := boxStyle.Render(body)

	if m.width == 0 {
		return box
	}
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
