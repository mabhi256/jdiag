package monitor

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines the key bindings
type KeyMap struct {
	Up            key.Binding
	Down          key.Binding
	Left          key.Binding
	Right         key.Binding
	Tab           key.Binding
	Refresh       key.Binding
	Quit          key.Binding
	SelectProcess key.Binding
	Reconnect     key.Binding
	Enter         key.Binding
	Escape        key.Binding
	PageUp        key.Binding
	PageDown      key.Binding
}

func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Tab, k.Refresh, k.Quit}
}

func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right},
		{k.Tab, k.Refresh, k.SelectProcess, k.Reconnect, k.Quit},
	}
}

var keys = KeyMap{
	Up:            key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:          key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	Left:          key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "left")),
	Right:         key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "right")),
	Tab:           key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "switch view")),
	Refresh:       key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
	Quit:          key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	SelectProcess: key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "select process")),
	Reconnect:     key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "reconnect")),
	Enter:         key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
	Escape:        key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	PageUp:        key.NewBinding(key.WithKeys("pgup"), key.WithHelp("pgup", "page up")),
	PageDown:      key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("pgdown", "page down")),
}

func (m *Model) scrollUp(lines int) {
	currentPos := m.scrollPositions[m.activeTab]
	newPos := currentPos - lines
	if newPos < 0 {
		newPos = 0
	}
	m.scrollPositions[m.activeTab] = newPos
}

func (m *Model) scrollDown(lines int) {
	currentPos := m.scrollPositions[m.activeTab]
	m.scrollPositions[m.activeTab] = currentPos + lines
	// Max scroll validation happens in applyScrolling()
}
