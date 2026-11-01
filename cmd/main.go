package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const footerHelp = "w/s — движение · a — вверх · d — вход · esc — выход"

type entry struct {
	name  string
	isDir bool
}

type model struct {
	dir     string
	entries []entry
	err     error
	cursor  int
	width   int
	height  int
}

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	dirStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	selectedStyle = lipgloss.NewStyle().Reverse(true)
	helpStyle     = lipgloss.NewStyle().Faint(true)
	errStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
)

func loadEntries(dir string) ([]entry, error) {
	des, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	entries := make([]entry, 0, len(des))
	for _, de := range des {
		entries = append(entries, entry{name: de.Name(), isDir: de.IsDir()})
	}
	return entries, nil
}

func (model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc", "ctrl+c":
			return m, tea.Quit
		case "w":
			m.moveCursor(-1)
		case "s":
			m.moveCursor(1)
		case "a":
			m.goParent()
		case "d":
			m.goChild()
		}
	}
	return m, nil
}

// goParent moves to the parent directory and keeps the cursor on the
// directory we just left. It does nothing at the filesystem root.
func (m *model) goParent() {
	parent := filepath.Dir(m.dir)
	if parent == m.dir {
		return
	}
	m.navigate(parent, filepath.Base(m.dir))
}

// goChild enters the directory under the cursor. It does nothing when the
// cursor points at a regular file.
func (m *model) goChild() {
	if len(m.entries) == 0 {
		return
	}
	e := m.entries[m.cursor]
	if !e.isDir {
		return
	}
	m.navigate(filepath.Join(m.dir, e.name), "")
}

// navigate reloads the listing for dir and optionally puts the cursor on the
// entry named selectName.
func (m *model) navigate(dir, selectName string) {
	entries, err := loadEntries(dir)
	if err != nil {
		m.err = err
		return
	}

	m.dir = dir
	m.entries = entries
	m.err = nil
	m.cursor = 0

	if selectName != "" {
		for i, e := range entries {
			if e.name == selectName && e.isDir {
				m.cursor = i
				break
			}
		}
	}
}

// sanitize makes a file name safe to draw: invalid UTF-8 becomes U+FFFD and
// control characters that would break the layout become spaces. Names on
// Linux are arbitrary bytes, so this cannot be skipped.
func sanitize(s string) string {
	s = strings.ToValidUTF8(s, "\uFFFD")
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return ' '
		}
		return r
	}, s)
}

// name is the current directory name shown in the left column.
func (m model) name() string {
	base := filepath.Base(m.dir)
	if base == "." {
		return sanitize(m.dir)
	}
	return sanitize(base)
}

func (m *model) moveCursor(delta int) {
	if len(m.entries) == 0 {
		return
	}
	m.cursor = min(max(m.cursor+delta, 0), len(m.entries)-1)
}

func (m model) displayName(e entry) string {
	if e.isDir {
		return sanitize(e.name) + "/"
	}
	return sanitize(e.name)
}

func (m model) footerText() string {
	if len(m.entries) == 0 {
		return footerHelp
	}
	return fmt.Sprintf("%d/%d · %s", m.cursor+1, len(m.entries), footerHelp)
}

func (m model) visibleLines() int {
	return max(m.height-2, 1)
}

// columnWidths returns the widths of the left (current directory) and right
// (file list) columns. The right column is computed from every entry so that
// it does not change width while scrolling.
func (m model) columnWidths() (int, int) {
	const gap = 3 // " / "

	left := lipgloss.Width(m.name())
	right := 0
	for _, e := range m.entries {
		right = max(right, lipgloss.Width(m.displayName(e)))
	}
	if len(m.entries) == 0 {
		right = lipgloss.Width("(пусто)")
	}

	// Keep the directory column from crowding out the list.
	if m.width > gap+2 {
		left = min(left, max(m.width/3, 1))
		right = min(right, max(m.width-left-gap, 1))
	}

	return max(left, 1), max(right, 1)
}

func (m model) View() tea.View {
	if m.err != nil {
		msg := errStyle.Render(m.err.Error()) + "\n\n" +
			helpStyle.Render("a — вверх · esc — выход")
		v := tea.NewView(lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, msg))
		v.AltScreen = true
		return v
	}

	leftW, rightW := m.columnWidths()
	visible := m.visibleLines()

	// The selected entry is always pinned to the middle row; the list itself
	// scrolls behind it (with blank rows at the beginning and the end).
	center := visible / 2
	start := m.cursor - center

	leftRows := make([]string, visible)
	sepRows := make([]string, visible)
	rightRows := make([]string, visible)
	for i := range visible {
		leftRows[i] = strings.Repeat(" ", leftW)
		sepRows[i] = " "
		rightRows[i] = strings.Repeat(" ", rightW)
	}

	leftRows[center] = titleStyle.Render(ansi.Truncate(m.name(), leftW, "…"))
	sepRows[center] = helpStyle.Render("/")

	for r := range visible {
		idx := start + r
		if idx < 0 || idx >= len(m.entries) {
			continue
		}
		rightRows[r] = m.renderEntry(idx, rightW)
	}
	if len(m.entries) == 0 {
		rightRows[center] = helpStyle.Width(rightW).Render("(пусто)")
	}

	columns := lipgloss.JoinHorizontal(
		lipgloss.Top,
		strings.Join(leftRows, "\n"),
		" ",
		strings.Join(sepRows, "\n"),
		" ",
		strings.Join(rightRows, "\n"),
	)

	footer := ansi.Truncate(m.footerText(), max(m.width, 1), "…")
	footerLine := helpStyle.Width(m.width).Align(lipgloss.Center).Render(footer)

	block := lipgloss.JoinVertical(lipgloss.Center, columns, "", footerLine)

	v := tea.NewView(lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, block))
	v.AltScreen = true
	return v
}

func (m model) renderEntry(i, width int) string {
	e := m.entries[i]

	name := ansi.Truncate(m.displayName(e), width, "…")

	style := lipgloss.NewStyle()
	switch {
	case i == m.cursor:
		style = selectedStyle
	case e.isDir:
		style = dirStyle
	}
	return style.Width(width).Render(name)
}

func main() {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	m := model{dir: dir}
	m.entries, m.err = loadEntries(dir)

	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
