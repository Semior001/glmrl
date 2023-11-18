package teax

import (
	"fmt"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/samber/lo"
	"golang.org/x/crypto/ssh/terminal"
	"log"
	"strings"
	"sync"
	"text/template"
	"time"
)

// Column is a column to show in the table.
type Column[T any] struct {
	table.Column
	Extract func(T) string
}

// Actor is a data source for a table.
type Actor[T any] interface {
	Load() ([]T, error)
	OnEnter(T) error
}

// Table is a table model.
type Table[T any] struct {
	table table.Model
	src   Actor[T]
	cols  []Column[T]
	data  struct {
		mu         sync.Mutex
		entries    []T
		lastReload time.Time
	}
}

// NewTable creates a new Table.
func NewTable[T any](cols []Column[T], act Actor[T]) (*Table[T], error) {
	tbl := &Table[T]{src: act, cols: cols}
	tbl.table = table.New()
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	tbl.table.SetStyles(s)

	log.Printf("[DEBUG] getting terminal size")
	width, height, err := terminal.GetSize(0)
	if err != nil {
		return nil, fmt.Errorf("get terminal size: %w", err)
	}

	log.Printf("[DEBUG] terminal size: %dx%d, setting to table", width, height)
	tbl.table.SetWidth(width)
	tbl.table.SetHeight(height)
	tbl.table.Focus()

	if err = tbl.redrawColumns(); err != nil {
		return nil, fmt.Errorf("redraw columns: %w", err)
	}

	if err = tbl.reload(); err != nil {
		return nil, fmt.Errorf("load for the first time: %w", err)
	}

	return tbl, nil
}

func (t *Table[T]) reload() error {
	t.data.mu.Lock()
	defer t.data.mu.Unlock()

	// do not allow to reload more often than once per 30 seconds
	if time.Since(t.data.lastReload) < 30*time.Second {
		t.table.UpdateViewport()
		return nil
	}

	t.data.lastReload = time.Now()

	entries, err := t.src.Load()
	if err != nil {
		return fmt.Errorf("load entries: %w", err)
	}
	t.data.entries = entries

	if len(t.data.entries) > 0 {
		t.table.SetRows(lo.Map(t.data.entries, func(entry T, _ int) table.Row {
			return lo.Map(t.cols, func(col Column[T], _ int) string {
				return col.Extract(entry)
			})
		}))
	}
	t.table.UpdateViewport()

	return nil
}

func (t *Table[T]) redrawColumns() error {
	data := struct {
		LastReload time.Time
		Total      int
	}{
		LastReload: t.data.lastReload,
		Total:      len(t.data.entries),
	}

	widthPerUnit := t.table.Width() / lo.Sum(lo.Map(t.cols, func(c Column[T], _ int) int { return c.Width }))

	cols := make([]table.Column, len(t.cols))
	for idx, col := range t.cols {
		tmpl, err := template.New("").Parse(col.Title)
		if err != nil {
			return fmt.Errorf("parse template: %w", err)
		}
		buf := &strings.Builder{}
		if err = tmpl.Execute(buf, data); err != nil {
			return fmt.Errorf("execute template: %w", err)
		}
		cols[idx] = table.Column{Title: buf.String(), Width: col.Width * widthPerUnit}
	}

	t.table.SetColumns(cols)
	return nil
}

// Init does nothing.
func (t *Table[T]) Init() tea.Cmd { return tea.ClearScreen }

// Update updates the table model.
func (t *Table[T]) Update(msg tea.Msg) (_ tea.Model, cmd tea.Cmd) {
	log.Printf("[DEBUG][TUI-Table] received message: %#v", msg)

	if msg, ok := msg.(tea.WindowSizeMsg); ok {
		t.table.SetWidth(msg.Width)
		t.table.SetHeight(msg.Height)
		log.Printf("[DEBUG][TUI-Table] resizing table to new window size: %dx%d", msg.Width, msg.Height)
		// force rerender
		return t, t.reloadCmd()
	}

	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "ctrl+c", "q":
			return t, tea.Quit
		case "enter":
			return t, t.enterCmd()
		case "r":
			return t, t.reloadCmd()
		}
	}

	t.table, cmd = t.table.Update(msg)
	return t, cmd
}

func (t *Table[T]) enterCmd() tea.Cmd {
	return func() tea.Msg {
		t.data.mu.Lock()
		defer t.data.mu.Unlock()

		if len(t.data.entries) == 0 {
			return nil
		}

		if err := t.src.OnEnter(t.data.entries[t.table.Cursor()]); err != nil {
			log.Printf("[ERROR][TUI-Table] OnEnter callback returned error: %v", err)
			return tea.Quit
		}

		return nil
	}
}

func (t *Table[T]) reloadCmd() tea.Cmd {
	return func() tea.Msg {
		if err := t.reload(); err != nil {
			log.Printf("[ERROR][TUI-Table] reload: %v", err)
			return tea.Quit
		}
		return nil
	}
}

// View renders the table.
func (t *Table[T]) View() string {
	t.data.mu.Lock()
	defer t.data.mu.Unlock()
	if err := t.redrawColumns(); err != nil {
		log.Printf("[ERROR][TUI-Table] redraw columns: %v", err)
		return fmt.Sprintf("failed to render table: %v", err)
	}
	return t.table.View()
}
