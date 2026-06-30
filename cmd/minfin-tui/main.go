// Command minfin-tui is a TUI interface to the minfin database
package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/zackb/minfin/internal/dashboard"
	"github.com/zackb/minfin/internal/env"
	"github.com/zackb/minfin/internal/store"
)

func main() {
	st, err := store.Open(env.DBPath())
	if err != nil {
		log.Fatal(err)
	}
	defer st.Close()

	d, err := dashboard.Load(st, time.Now())
	if err != nil {
		log.Fatal(err)
	}
	if _, err := tea.NewProgram(model{st: st, d: d}).Run(); err != nil {
		log.Fatal(err)
	}
}

// for now loads once at startup; 'r' reloads. No live tail of the DB
type model struct {
	st  *store.Store
	d   dashboard.Dashboard
	err error
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "r":
			m.d, m.err = dashboard.Load(m.st, time.Now())
		}
	}
	return m, nil
}

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	labelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	posStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	negStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	hintStyle  = lipgloss.NewStyle().Faint(true)
)

func (m model) View() string {
	if m.err != nil {
		return "error: " + m.err.Error() + "\n"
	}
	if m.d.Empty() {
		return "No portfolio yet — sync one with the server first.\n\n" + hintStyle.Render("q quit") + "\n"
	}

	var b strings.Builder
	fmt.Fprintln(&b, titleStyle.Render("minfin · "+m.d.Portfolio.Name))
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "%s  %s\n", labelStyle.Render("Net worth "), money(m.d.NetWorth))
	fmt.Fprintf(&b, "%s  %s\n", labelStyle.Render("Assets    "), money(m.d.Assets))
	fmt.Fprintf(&b, "%s  %s\n", labelStyle.Render("Liabilities"), money(m.d.Liabilities))
	fmt.Fprintln(&b)

	for _, a := range m.d.Accounts {
		name := a.Display()
		if len(name) > 28 {
			name = name[:27] + "…"
		}
		fmt.Fprintf(&b, "  %-28s %14s\n", name, money(a.Balance))
	}

	fmt.Fprintln(&b)
	fmt.Fprintln(&b, hintStyle.Render("r refresh · q quit"))
	return b.String()
}

func money(f float64) string {
	s := dashboard.USD(f)
	if f < 0 {
		return negStyle.Render(s)
	}
	return posStyle.Render(s)
}
