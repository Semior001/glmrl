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
	data  struct {
		mu         sync.Mutex
		entries    []T
		lastReload time.Time
		loadedIn   time.Duration
	}
	TableParams[T]
}

// TableParams are the parameters to initialize a Table.
type TableParams[T any] struct {
	Columns        []Column[T]
	Actor          Actor[T]
	PollInterval   time.Duration
	BorrowedHeight int // table will cut off these lines from the top at render
}

// NewTable creates a new Table.
func NewTable[T any](params TableParams[T]) (*Table[T], error) {
	tbl := &Table[T]{TableParams: params}
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
	tbl.resize(width, height)

	if err = tbl.redrawColumns(); err != nil {
		return nil, fmt.Errorf("redraw columns: %w", err)
	}

	if _, err = tbl.reload(); err != nil {
		return nil, fmt.Errorf("load for the first time: %w", err)
	}

	return tbl, nil
}

// Focus focuses the table.
func (t *Table[T]) Focus() { t.table.Focus() }

// Init does nothing.
func (t *Table[T]) Init() tea.Cmd { return t.scheduleTick() }

// Update updates the table model.
func (t *Table[T]) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	log.Printf("[DEBUG][TUI-Table] received message: %#v", msg)

	if _, ok := msg.(tickMsg); ok {
		return t, tea.Batch(t.reloadCmd(), t.scheduleTick())
	}

	if msg, ok := msg.(tea.WindowSizeMsg); ok {
		t.resize(msg.Width, msg.Height)

		log.Printf("[DEBUG][TUI-Table] resizing table to new window size: %dx%d", msg.Width, msg.Height)
		return t, tea.ClearScreen
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

	var cmd tea.Cmd
	t.table, cmd = t.table.Update(msg)
	return t, cmd
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

func (t *Table[T]) resize(w, h int) {
	t.table.SetWidth(w)
	t.table.SetHeight(h - 2 - t.BorrowedHeight) // cut off the status bar and the borrowed height
}

func (t *Table[T]) reload() (updated bool, err error) {
	t.data.mu.Lock()
	defer t.data.mu.Unlock()

	// do not allow to reload more often than once per 30 seconds
	//if time.Since(t.data.lastReload) < 30*time.Second {
	//	return false, nil
	//}

	t.data.lastReload = time.Now()
	start := time.Now()

	entries, err := t.Actor.Load()
	if err != nil {
		return false, fmt.Errorf("load entries: %w", err)
	}
	t.data.entries = entries

	if len(t.data.entries) > 0 {
		t.table.SetRows(lo.Map(t.data.entries, func(entry T, _ int) table.Row {
			return lo.Map(t.Columns, func(col Column[T], _ int) string {
				return col.Extract(entry)
			})
		}))
	}
	t.data.loadedIn = time.Since(start)
	t.data.loadedIn = t.data.loadedIn.Round(100 * time.Millisecond)

	return true, nil
}

func (t *Table[T]) redrawColumns() error {
	type columnData struct {
		LastReload time.Time
		LoadedIn   time.Duration
		Total      int
	}

	widthPerUnit := t.table.Width() / lo.Sum(lo.Map(t.Columns, func(c Column[T], _ int) int { return c.Width }))

	data := columnData{LastReload: t.data.lastReload, LoadedIn: t.data.loadedIn, Total: len(t.data.entries)}
	cols := make([]table.Column, len(t.Columns))
	for idx, col := range t.Columns {
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

func (t *Table[T]) enterCmd() tea.Cmd {
	return func() tea.Msg {
		t.data.mu.Lock()
		defer t.data.mu.Unlock()

		if len(t.data.entries) == 0 {
			return nil
		}

		if err := t.Actor.OnEnter(t.data.entries[t.table.Cursor()]); err != nil {
			log.Printf("[ERROR][TUI-Table] OnEnter callback returned error: %v", err)
			return tea.Quit
		}

		return nil
	}
}

func (t *Table[T]) reloadCmd() tea.Cmd {
	return func() tea.Msg {
		upd, err := t.reload()
		if err != nil {
			log.Printf("[ERROR][TUI-Table] reload: %v", err)
			return tea.Quit
		}
		if upd {
			return tea.ClearScreen()
		}
		return nil
	}
}

func (t *Table[T]) scheduleTick() tea.Cmd {
	return tea.Tick(t.PollInterval, func(time.Time) tea.Msg { return tickMsg{} })
}
