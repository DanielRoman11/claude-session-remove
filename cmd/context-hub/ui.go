package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/DanielRoman11/context-hub/internal/agents"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Catppuccin (Mocha for dark terminals, Latte for light ones) — the palette
// most TUI tools like lazygit reach for today. Each provider gets its own
// accent so its sessions are recognizable at a glance.
//
// Icons: none of these three tools has an icon in any released font
// (Nerd Font or otherwise) — see displayIcon below for Claude Code's case
// specifically — so all three use the closest Unicode approximation of
// their real mark: a sunburst/asterisk for Claude Code, a modular
// pixel-block grid for OpenCode, and a crescent moon for Kimi Code
// (Moonshot AI's name literally means "the dark side of the moon").
var (
	colorAccent   = lipgloss.AdaptiveColor{Light: "#8839ef", Dark: "#cba6f7"} // mauve
	colorOpenCode = lipgloss.AdaptiveColor{Light: "#1e66f5", Dark: "#89b4fa"} // blue
	colorKimi     = lipgloss.AdaptiveColor{Light: "#179299", Dark: "#94e2d5"} // teal
	colorText     = lipgloss.AdaptiveColor{Light: "#4c4f69", Dark: "#cdd6f4"} // text
	colorMuted    = lipgloss.AdaptiveColor{Light: "#6c6f85", Dark: "#a6adc8"} // subtext0
	colorFaint    = lipgloss.AdaptiveColor{Light: "#9ca0b0", Dark: "#6c7086"} // overlay0
	colorDanger   = lipgloss.AdaptiveColor{Light: "#d20f39", Dark: "#f38ba8"} // red
	colorSuccess  = lipgloss.AdaptiveColor{Light: "#40a02b", Dark: "#a6e3a1"} // green
	colorOnAcc    = lipgloss.AdaptiveColor{Light: "#eff1f5", Dark: "#1e1e2e"} // base

	borderStyle   = lipgloss.NewStyle().Foreground(colorAccent)
	appTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
	subtitleStyle = lipgloss.NewStyle().Foreground(colorMuted)

	cursorStyle = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)

	itemStyle = lipgloss.NewStyle().Foreground(colorText).Bold(true)

	selectedItemStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorOnAcc).
				Background(colorAccent)

	currentTagStyle = lipgloss.NewStyle().Foreground(colorSuccess).Italic(true)
	metaStyle       = lipgloss.NewStyle().Foreground(colorMuted)
	timeStyle       = lipgloss.NewStyle().Foreground(colorFaint)
	dimStyle        = lipgloss.NewStyle().Foreground(colorMuted)

	helpKeyStyle  = lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	helpDescStyle = lipgloss.NewStyle().Foreground(colorMuted)

	buttonStyle = lipgloss.NewStyle().Padding(0, 2).MarginRight(2).Foreground(colorMuted)

	buttonDangerStyle = lipgloss.NewStyle().Padding(0, 2).MarginRight(2).Bold(true).
				Foreground(colorOnAcc).Background(colorDanger)

	successStyle = lipgloss.NewStyle().Bold(true).Foreground(colorSuccess)
)

// providerColor picks each provider's accent by name, so the agents package
// doesn't need to depend on lipgloss at all.
func providerColor(name string) lipgloss.AdaptiveColor {
	switch name {
	case "OpenCode":
		return colorOpenCode
	case "Kimi Code":
		return colorKimi
	default:
		return colorAccent
	}
}

// displayIcon returns the glyph to render for a provider. Codicons does
// define a real Claude mark (U+EC82), but it's new enough that no released
// Nerd Font build actually ships it yet (checked against an installed
// Maple Mono NF build: its Codicons range ends at U+EC1E, before U+EC82),
// so it would render as a blank box for effectively everyone. Until a
// released Nerd Font build includes it, all three providers use a plain
// Unicode glyph instead.
func displayIcon(p agents.Provider) string {
	return p.Icon()
}

type screen int

const (
	screenList screen = iota
	screenConfirm
)

type model struct {
	sessions  []agents.Session
	cursor    int
	screen    screen
	currentID string
	target    agents.Session
	action    string // "resume" or "delete", set once the program quits
	width     int
	height    int
}

func newModel(sessions []agents.Session, currentID string) model {
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
			case "enter", "o":
				if len(m.sessions) > 0 {
					m.target = m.sessions[m.cursor]
					m.action = "resume"
					return m, tea.Quit
				}
			case "d":
				if len(m.sessions) > 0 {
					m.target = m.sessions[m.cursor]
					m.screen = screenConfirm
				}
			}
		case screenConfirm:
			switch msg.String() {
			case "y", "enter":
				m.action = "delete"
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

// Layout constants for a session card:
//
//	✳  Fix the very very long authentication flow bug
//	   Claude Code · Sep 16 · current                        2h ago
//
// gutter (cursor marker) + icon + space line up the title on row 1; the
// same width of plain indent lines up the meta row underneath it.
const (
	gutterWidth  = 4 // "› " or "  " (2) + icon (1) + space (1)
	timeColWidth = 8 // widest relTime() output ("59m ago") plus a gap
	rowsPerCard  = 3 // title line + meta line + blank separator
)

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

func padRight(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

// frame draws a full lazygit-style panel: a title (and optional right-hand
// label) embedded in the top border, a body, and an optional footer split
// off by a divider. It always renders exactly width x height.
func frame(width, height int, title, rightLabel string, body, footer []string) string {
	if width < 24 {
		width = 24
	}
	if height < 6 {
		height = 6
	}
	contentWidth := width - 4 // "│ " + content + " │"

	var out []string
	out = append(out, topBorder(width, title, rightLabel))

	footerBlock := 0
	if len(footer) > 0 {
		footerBlock = len(footer) + 1 // +1 for the divider
	}
	bodyHeight := height - 2 - footerBlock
	if bodyHeight < 1 {
		bodyHeight = 1
	}

	for i := 0; i < bodyHeight; i++ {
		content := ""
		if i < len(body) {
			content = body[i]
		}
		out = append(out, contentLine(content, contentWidth))
	}

	if len(footer) > 0 {
		out = append(out, borderStyle.Render("├"+strings.Repeat("─", width-2)+"┤"))
		for _, f := range footer {
			out = append(out, contentLine(f, contentWidth))
		}
	}

	out = append(out, borderStyle.Render("╰"+strings.Repeat("─", width-2)+"╯"))
	return strings.Join(out, "\n")
}

func topBorder(width int, title, rightLabel string) string {
	titleStyled := appTitleStyle.Render(" " + title + " ")
	rightStyled := ""
	rightW := 0
	if rightLabel != "" {
		rightStyled = subtitleStyle.Render(" " + rightLabel + " ")
		rightW = lipgloss.Width(rightStyled)
	}
	fillLen := width - 4 - lipgloss.Width(titleStyled) - rightW
	if fillLen < 0 {
		fillLen = 0
	}
	return borderStyle.Render("╭─") + titleStyled + borderStyle.Render(strings.Repeat("─", fillLen)) + rightStyled + borderStyle.Render("─╮")
}

func contentLine(content string, contentWidth int) string {
	return borderStyle.Render("│") + " " + padRight(content, contentWidth) + " " + borderStyle.Render("│")
}

func (m model) viewList() string {
	width, height := m.width, m.height
	if width < 40 {
		width = 100
	}
	if height < 10 {
		height = 32
	}
	contentWidth := width - 4
	titleWidth := contentWidth - gutterWidth
	metaWidth := contentWidth - gutterWidth

	footer := []string{
		helpKeyStyle.Render("↑/↓") + " " + helpDescStyle.Render("navigate") + "    " +
			helpKeyStyle.Render("enter/o") + " " + helpDescStyle.Render("open") + "    " +
			helpKeyStyle.Render("d") + " " + helpDescStyle.Render("delete") + "    " +
			helpKeyStyle.Render("q") + " " + helpDescStyle.Render("quit"),
	}

	bodyHeight := height - 2 - (len(footer) + 1)
	if bodyHeight < 1 {
		bodyHeight = 1
	}
	visibleCards := bodyHeight / rowsPerCard
	if visibleCards < 1 {
		visibleCards = 1
	}
	scroll := 0
	if m.cursor >= visibleCards {
		scroll = m.cursor - visibleCards + 1
	}

	var body []string
	end := scroll + visibleCards
	if end > len(m.sessions) {
		end = len(m.sessions)
	}
	for i := scroll; i < end; i++ {
		s := m.sessions[i]
		selected := i == m.cursor

		icon := lipgloss.NewStyle().Foreground(providerColor(s.Provider.Name())).Render(displayIcon(s.Provider))
		titleText := padRight(truncateEllipsis(s.Title, titleWidth), titleWidth)

		metaLeftPlain := s.Provider.Name()
		if !s.CreatedAt.IsZero() {
			metaLeftPlain += " · " + s.CreatedAt.Format("Jan 2")
		}
		tag := ""
		if isCurrent(s, m.currentID) {
			tag = " · current"
		}
		timeStr := padRight(relTime(s.UpdatedAt), timeColWidth)
		pad := metaWidth - timeColWidth - lipgloss.Width(metaLeftPlain) - lipgloss.Width(tag)
		if pad < 1 {
			pad = 1
		}

		cursor := "  "
		var titleLine, metaLine string
		if selected {
			cursor = cursorStyle.Render("› ")
			titleLine = cursor + icon + " " + selectedItemStyle.Render(titleText)
			metaLine = "   " + selectedItemStyle.Render(metaLeftPlain+tag+strings.Repeat(" ", pad)+timeStr)
		} else {
			titleLine = cursor + icon + " " + itemStyle.Render(titleText)
			metaLine = "   " + metaStyle.Render(metaLeftPlain) + currentTagStyle.Render(tag) +
				strings.Repeat(" ", pad) + timeStyle.Render(timeStr)
		}

		body = append(body, titleLine, metaLine, "")
	}
	for len(body) < bodyHeight {
		body = append(body, "")
	}

	rightLabel := fmt.Sprintf("%d sessions", len(m.sessions))
	return frame(width, height, "context-hub", rightLabel, body, footer)
}

func (m model) viewConfirm() string {
	width, height := m.width, m.height
	if width < 40 {
		width = 100
	}
	if height < 10 {
		height = 32
	}

	panelWidth := 56
	panelHeight := 9
	contentWidth := panelWidth - 4

	name := itemStyle.Render(padRight(truncateEllipsis(m.target.Title, contentWidth), contentWidth))
	provider := lipgloss.NewStyle().Foreground(providerColor(m.target.Provider.Name())).Render(
		displayIcon(m.target.Provider) + " " + m.target.Provider.Name())

	body := []string{
		"",
		name,
		provider,
	}
	footer := []string{
		lipgloss.JoinHorizontal(lipgloss.Top,
			buttonDangerStyle.Render("y Delete"),
			buttonStyle.Render("n Cancel"),
		),
	}

	panel := frame(panelWidth, panelHeight, "Delete session?", "", body, footer)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, panel)
}

func relTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	default:
		return t.Format("Jan 2")
	}
}
