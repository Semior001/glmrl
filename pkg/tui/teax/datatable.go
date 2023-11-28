package teax

import (
	"fmt"
	"github.com/charmbracelet/bubbles/key"
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

// Column is a column to show in the table. It also contains
// a function to extract the value from the data source.
type Column[T any] struct {
	table.Column
	Extract func(T) string
}

// Actor is a data source for a table.
type Actor[T any] interface {
	Load() ([]T, error)
	// OnKey is called when a key is pressed on a row.
	// Note: key might be a set of keys, e.g. "ctrl+c", it is important to
	// consider all possible combinations.
	// It is never called on "r", "ctrl+c" or "q" key presses, as they're
	// handled by the table itself.
	OnKey(key string, row int, val T) (hide bool, err error)
}

// RefreshingDataTable is a table, that loads its data from an
// Actor with periodic updates, or on demand.
type RefreshingDataTable[T any] struct {
	table table.Model
	data  struct {
		mu         sync.Mutex
		entries    []T
		lastReload time.Time
		loadedIn   time.Duration
	}
	RefreshingDataTableParams[T]
}

// RefreshingDataTableParams are the parameters to initialize a RefreshingDataTable.
type RefreshingDataTableParams[T any] struct {
	Columns        []Column[T]
	Actor          Actor[T]
	PollInterval   time.Duration
	BorrowedHeight int // table will cut off these lines from the top at render
}

// NewRefreshingDataTable creates a new RefreshingDataTable.
func NewRefreshingDataTable[T any](params RefreshingDataTableParams[T]) (*RefreshingDataTable[T], error) {
	tbl := &RefreshingDataTable[T]{RefreshingDataTableParams: params}
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
func (t *RefreshingDataTable[T]) Focus() { t.table.Focus() }

// Init does nothing.
func (t *RefreshingDataTable[T]) Init() tea.Cmd { return t.scheduleTick() }

// Update updates the table model.
func (t *RefreshingDataTable[T]) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	log.Printf("[DEBUG][TUI-RefreshingDataTable] received message: %#v", msg)

	if _, ok := msg.(tickMsg); ok {
		return t, tea.Batch(t.reloadCmd(), t.scheduleTick())
	}

	if msg, ok := msg.(tea.WindowSizeMsg); ok {
		t.resize(msg.Width, msg.Height)

		log.Printf("[DEBUG][TUI-RefreshingDataTable] resizing table to new window size: %dx%d", msg.Width, msg.Height)
		return t, tea.ClearScreen
	}

	if msg, ok := msg.(tea.KeyMsg); ok {
		// if key is meant to be processed by the table, don't do anything
		tblKey := []bool{
			key.Matches(msg, table.DefaultKeyMap().LineUp),
			key.Matches(msg, table.DefaultKeyMap().LineDown),
			key.Matches(msg, table.DefaultKeyMap().PageUp),
			key.Matches(msg, table.DefaultKeyMap().PageDown),
			key.Matches(msg, table.DefaultKeyMap().HalfPageUp),
			key.Matches(msg, table.DefaultKeyMap().HalfPageDown),
			key.Matches(msg, table.DefaultKeyMap().LineDown),
			key.Matches(msg, table.DefaultKeyMap().GotoTop),
			key.Matches(msg, table.DefaultKeyMap().GotoBottom),
		}

		for _, k := range tblKey {
			if k {
				var cmd tea.Cmd
				t.table, cmd = t.table.Update(msg)
				return t, cmd
			}
		}

		switch msg.String() {
		case "ctrl+c", "c+ctrl", "q":
			return t, tea.Quit
		case "r":
			return t, t.reloadCmd()
		default:
			return t, t.keyCmd(msg.String())
		}
	}

	log.Printf("[DEBUG][TUI-RefreshingDataTable] unhandled message: %#v", msg)
	return t, nil
}

// View renders the table.
func (t *RefreshingDataTable[T]) View() string {
	t.data.mu.Lock()
	defer t.data.mu.Unlock()
	if err := t.redrawColumns(); err != nil {
		log.Printf("[ERROR][TUI-RefreshingDataTable] redraw columns: %v", err)
		return fmt.Sprintf("failed to render table: %v", err)
	}
	return t.table.View()
}

func (t *RefreshingDataTable[T]) reload() (updated bool, err error) {
	t.data.mu.Lock()
	defer t.data.mu.Unlock()

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

func (t *RefreshingDataTable[T]) hide(idx int) {
	t.data.mu.Lock()
	defer t.data.mu.Unlock()

	t.data.entries = append(t.data.entries[:idx], t.data.entries[idx+1:]...)

	if len(t.data.entries) > 0 {
		t.table.SetRows(lo.Map(t.data.entries, func(entry T, _ int) table.Row {
			return lo.Map(t.Columns, func(col Column[T], _ int) string {
				return col.Extract(entry)
			})
		}))
	}
}

func (t *RefreshingDataTable[T]) entry(cursor int) (v T, ok bool) {
	t.data.mu.Lock()
	defer t.data.mu.Unlock()

	if len(t.data.entries) == 0 {
		return v, false
	}

	if len(t.data.entries) <= cursor || cursor < 0 {
		return v, false
	}

	return t.data.entries[cursor], true
}

func (t *RefreshingDataTable[T]) resize(w, h int) {
	t.table.SetWidth(w)
	t.table.SetHeight(h - 2 - t.BorrowedHeight) // cut off the status bar and the borrowed height
}

func (t *RefreshingDataTable[T]) redrawColumns() error {
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

func (t *RefreshingDataTable[T]) keyCmd(key string) tea.Cmd {
	return func() tea.Msg {
		cursor := t.table.Cursor()

		entry, ok := t.entry(cursor)
		if !ok {
			log.Printf("[ERROR][TUI-RefreshingDataTable] cursor is out of bounds: %d", cursor)
			return nil
		}

		hide, err := t.Actor.OnKey(key, cursor, entry)
		if err != nil {
			log.Printf("[ERROR][TUI-RefreshingDataTable] OnEnter callback returned error: %v", err)
			return tea.Quit
		}

		// we rather hide the entry instead of reloading the whole table, because reload
		// takes time, and we don't want to block the UI for a long time
		if hide {
			log.Printf("[DEBUG][TUI-RefreshingDataTable] hiding entry at %d", cursor)
			t.hide(cursor)
		}

		return nil
	}
}

func (t *RefreshingDataTable[T]) reloadCmd() tea.Cmd {
	return func() tea.Msg {
		upd, err := t.reload()
		if err != nil {
			log.Printf("[ERROR][TUI-RefreshingDataTable] reload: %v", err)
			return tea.Quit
		}
		if upd {
			return tea.ClearScreen()
		}
		return nil
	}
}

func (t *RefreshingDataTable[T]) scheduleTick() tea.Cmd {
	if t.PollInterval == 0 {
		return nil
	}
	return tea.Tick(t.PollInterval, func(time.Time) tea.Msg { return tickMsg{} })
}
