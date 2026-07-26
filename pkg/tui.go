// pkg/tui.go

package pkg

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

//- Types ------------------------------------------------------------------------------------------

// NotifyColors holds the hex colours a notification is drawn with, taken from
// the [colors] section of the config. An empty Footer hides the footer.
type NotifyColors struct {
	Border    string
	Head      string
	Message   string
	Author    string
	Selection string
	Footer    string

	// Expired is the colour a notification is drawn in once its caller has
	// stopped waiting for an answer.
	Expired string
}

// Notification is everything the TUI needs to draw a single notification.
//
// With no Options and no Custom it is a plain message the user can only
// dismiss. Otherwise it becomes a picker: Options are listed and one is chosen
// with enter, Multiple lets several be toggled with space first, and Custom
// adds a final row the user can type their own answer into.
type Notification struct {
	Author  string
	Head    string
	Body    string
	Options []string

	Custom   bool
	Multiple bool

	// Expired marks a notification whose caller gave up waiting. It keeps what
	// it said, but there is no longer anyone to answer, so it is drawn as a
	// plain message that can only be thrown away.
	Expired bool

	Colors NotifyColors
}

// NotifyResult is the outcome of showing a notification. Selected holds the
// chosen options — at most one unless Multiple was set — and is only populated
// for ActionSelect.
type NotifyResult struct {
	Action   string
	Selected []string
}

// notifyModel is the bubbletea model that draws a notification and tracks what
// the user is picking.
type notifyModel struct {
	notification Notification

	// cursor indexes Options, except when it equals len(Options) and Custom is
	// set, where it sits on the text input.
	cursor  int
	checked []bool
	input   textinput.Model

	// inactive marks a notification the user is not currently interacting with
	// — a box in the side panel they have not entered — so it is drawn without
	// a cursor and takes no typing.
	inactive bool

	width  int
	height int

	result NotifyResult
}

// Actions returned by RunNotifyTUI, describing how the user dismissed the
// notification.
const (
	ActionIgnore = "ignore"
	ActionClear  = "clear"
	ActionSelect = "select"

	// ActionTimeout is not something the user does: it is what became of a
	// notification whose caller stopped waiting before anyone answered.
	ActionTimeout = "timeout"
)

const (
	// Blank rows above and below, and blank columns either side, of the text.
	// The tmux popup draws the border, so the TUI only insets from it.
	padY = 1
	padX = 2

	// Blank rows between the message and the footer.
	footerGap = 1

	// MinWidth is the fewest columns the TUI can be drawn into: the padding
	// either side and a single column of text.
	MinWidth = padX*2 + 1

	cursorMark   = "❯ "
	noCursorMark = "  "
)

//- Private Helpers --------------------------------------------------------------------------------

// Whether this notification asks the user to pick something, rather than just
// telling them about it.
func (n Notification) interactive() bool {
	// Nothing is left to pick once the caller has gone; the options would only
	// offer an answer that has nowhere to go.
	return !n.Expired && (len(n.Options) > 0 || n.Custom)
}

// Set up the model for a notification, ready to be run or measured.
func newNotifyModel(n Notification) notifyModel {
	input := textinput.New()
	input.Prompt = ""
	input.Placeholder = "type your own…"

	m := notifyModel{
		notification: n,
		checked:      make([]bool, len(n.Options)),
		input:        input,
		result:       NotifyResult{Action: ActionIgnore},
	}

	// With no options at all, the text input is the only thing to land on.
	if n.Custom && len(n.Options) == 0 {
		m.input.Focus()
	}

	return m
}

// Whether the cursor is sitting on the custom text input rather than an option.
func (m notifyModel) onInput() bool {
	return !m.inactive && m.notification.Custom && m.cursor == len(m.notification.Options)
}

// Take the user's attention away from this notification, or give it back. Only
// the notification being interacted with may hold a blinking text cursor, so
// the input follows the model in and out of use.
func (m *notifyModel) setActive(active bool) {
	m.inactive = !active

	if m.onInput() {
		m.input.Focus()
	} else {
		m.input.Blur()
	}
}

// The number of rows the user can move between.
func (m notifyModel) rowCount() int {
	// An expired notification has nothing to move between: what it asked is
	// still worth reading, but there is nobody left to take the answer.
	if m.notification.Expired {
		return 0
	}
	if m.notification.Custom {
		return len(m.notification.Options) + 1
	}
	return len(m.notification.Options)
}

// Move the cursor by delta rows, wrapping at either end.
func (m *notifyModel) moveCursor(delta int) {
	count := m.rowCount()
	if count == 0 {
		return
	}

	m.cursor = (m.cursor + delta + count) % count

	// The input only accepts typing while the cursor is actually on it.
	if m.onInput() {
		m.input.Focus()
	} else {
		m.input.Blur()
	}
}

// Gather the answer for enter: the typed text when on the input, every ticked
// option when selecting multiple, or the single option under the cursor —
// which is also what selecting multiple falls back to when none are ticked.
func (m notifyModel) selection() []string {
	typed := strings.TrimSpace(m.input.Value())

	if !m.notification.Multiple {
		if m.onInput() {
			if typed == "" {
				return nil
			}
			return []string{typed}
		}
		return []string{m.notification.Options[m.cursor]}
	}

	selected := []string{}
	for i, option := range m.notification.Options {
		if m.checked[i] {
			selected = append(selected, option)
		}
	}
	if typed != "" {
		selected = append(selected, typed)
	}

	// Nothing ticked and the cursor sitting on an option: enter means that
	// option. Reading it as "no answer" instead leaves the key doing nothing
	// whatsoever, which cannot be told apart from the notification being stuck.
	if len(selected) == 0 && !m.onInput() {
		selected = append(selected, m.notification.Options[m.cursor])
	}

	return selected
}

// The keys offered at the bottom of the popup, longest wording first, so a
// narrow popup can fall back to a shorter hint rather than wrap the footer.
func (m notifyModel) footerVariants() []string {
	if !m.notification.interactive() {
		return []string{"[esc] ignore  [del] clear"}
	}

	// j/k would be swallowed by the text input, so only advertise them when
	// there is no typing to be done.
	move := []string{"[↑↓ jk] move", "[↑↓] move"}
	if m.notification.Custom {
		move = []string{"[↑↓] move"}
	}

	confirm := []string{"[enter] select", "[enter] pick"}
	if m.notification.Multiple {
		confirm = []string{"[enter] confirm", "[enter] done"}
	}

	variants := []string{}
	for i := range 2 {
		keys := []string{}
		if m.rowCount() > 1 {
			keys = append(keys, move[min(i, len(move)-1)])
		}
		if m.notification.Multiple {
			keys = append(keys, "[space] toggle")
		}
		keys = append(keys, confirm[min(i, len(confirm)-1)], "[esc] ignore")

		variants = append(variants, strings.Join(keys, "  "))
	}

	return variants
}

// The footer as it should be drawn at the given text width.
func (m notifyModel) footerText(textWidth int) string {
	variants := m.footerVariants()

	for _, variant := range variants {
		if lipgloss.Width(variant) <= textWidth {
			return variant
		}
	}

	return variants[len(variants)-1]
}

// Render the option list, one row per option plus the text input when Custom is
// set. Rows are numbered from 1, padded so the labels line up.
func (m notifyModel) optionRows(textWidth int) []string {
	accent := lipgloss.NewStyle().Foreground(lipgloss.Color(m.notification.Colors.Selection)).Bold(true)
	plain := lipgloss.NewStyle().Foreground(lipgloss.Color(m.notification.Colors.Message))

	indexWidth := len(strconv.Itoa(m.rowCount()))
	rows := make([]string, 0, m.rowCount())

	for i := range m.rowCount() {
		onRow := i == m.cursor && !m.inactive

		prefix := noCursorMark
		if onRow {
			prefix = cursorMark
		}
		prefix += fmt.Sprintf("%*d. ", indexWidth, i+1)

		// The last row is the text input when the user may type their own.
		label := ""
		if i < len(m.notification.Options) {
			label = m.notification.Options[i]
		} else {
			m.input.Width = max(textWidth-lipgloss.Width(prefix)-1, 1)
			label = m.input.View()
		}

		style := plain
		if onRow {
			style = accent
		}

		// Indent wrapped lines to sit under the label rather than the number.
		labelStyle := lipgloss.NewStyle().Width(textWidth - lipgloss.Width(prefix))

		// Ticked options are shown by colour rather than a marker. The text
		// input brings its own styling, so it is left alone.
		if i < len(m.notification.Options) {
			colour := m.notification.Colors.Message
			if m.notification.Multiple && m.checked[i] {
				colour = m.notification.Colors.Selection
			}
			labelStyle = labelStyle.Foreground(lipgloss.Color(colour)).Bold(onRow)
		}

		rows = append(rows, style.Render(prefix)+labelStyle.Render(label))
	}

	return rows
}

// Render the notification's text as the block above the footer, plus the footer
// itself, both wrapped to the given text width — the room left for words once
// whatever is drawing them has taken its padding and border.
func (m notifyModel) sections(textWidth int) (top, footer string) {
	headStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(m.notification.Colors.Head)).
		Bold(true).
		Width(textWidth)

	bodyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(m.notification.Colors.Message)).
		Width(textWidth)

	authorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(m.notification.Colors.Author)).
		Width(textWidth)

	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(m.notification.Colors.Footer)).
		Width(textWidth).
		Align(lipgloss.Center)

	// Who sent it and what it is about sit together above the message.
	header := []string{}
	if m.notification.Author != "" {
		header = append(header, authorStyle.Render(m.notification.Author))
	}
	if m.notification.Head != "" {
		header = append(header, headStyle.Render(m.notification.Head))
	}

	parts := []string{}
	if len(header) > 0 {
		parts = append(parts, append(header, "")...)
	}
	parts = append(parts, bodyStyle.Render(m.notification.Body))

	if rows := m.optionRows(textWidth); len(rows) > 0 {
		parts = append(parts, "")
		parts = append(parts, rows...)
	}

	top = lipgloss.JoinVertical(lipgloss.Left, parts...)

	// A footer colour of "<hidden>" leaves it off altogether.
	if m.notification.Colors.Footer == "" {
		return top, ""
	}

	return top, footerStyle.Render(m.footerText(textWidth))
}

// Start the model, blinking the cursor when there is a text input to type into.
func (m notifyModel) Init() tea.Cmd {
	if m.notification.Custom {
		return textinput.Blink
	}
	return nil
}

// Handle a keypress or resize, recording the outcome when the user is done.
func (m notifyModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "ctrl+c":
			m.result = NotifyResult{Action: ActionIgnore}
			return m, tea.Quit

		case "up":
			m.moveCursor(-1)
			return m, nil

		case "down":
			m.moveCursor(1)
			return m, nil

		case "enter":
			if !m.notification.interactive() {
				return m, nil
			}
			// Nothing to report: an empty text box, or no boxes ticked.
			selected := m.selection()
			if len(selected) == 0 {
				return m, nil
			}
			m.result = NotifyResult{Action: ActionSelect, Selected: selected}
			return m, tea.Quit
		}

		// While the cursor is on the text input every remaining key is typing,
		// so the single-letter shortcuts below have to stand aside.
		if m.onInput() {
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}

		switch msg.String() {
		case "k":
			m.moveCursor(-1)
		case "j":
			m.moveCursor(1)
		case " ":
			if m.notification.Multiple && m.cursor < len(m.checked) {
				m.checked[m.cursor] = !m.checked[m.cursor]
			}
		case "delete", "backspace":
			m.result = NotifyResult{Action: ActionClear}
			return m, tea.Quit
		}
	}

	return m, nil
}

// Draw the notification into the popup, keeping the footer against the bottom.
func (m notifyModel) View() string {
	// Nothing sensible to draw until tmux has told us how big the popup is.
	if m.width < 1 || m.height < 1 {
		return ""
	}

	top, footer := m.sections(max(m.width-padX*2, 1))
	body := max(m.height-padY*2, 1)

	// Rows the footer costs, if it is shown at all.
	footerRows := 0
	if footer != "" {
		footerRows = footerGap + lipgloss.Height(footer)
	}

	// A max_height can cut the popup short; trim the message rather than let it
	// push the footer off the bottom.
	if room := body - footerRows; lipgloss.Height(top) > room {
		top = strings.Join(strings.Split(top, "\n")[:max(room, 1)], "\n")
	}

	content := top
	if footer != "" {
		// The popup is normally sized to the text, but a min_height in the
		// config can leave slack — keep the footer against the bottom either
		// way. gap counts blank rows, so it takes one more newline to draw.
		gap := max(body-lipgloss.Height(top)-lipgloss.Height(footer), footerGap)
		content = top + strings.Repeat("\n", gap+1) + footer
	}

	return lipgloss.NewStyle().
		Padding(padY, padX).
		Width(m.width).
		Height(m.height).
		Render(content)
}

//- Public Calls -----------------------------------------------------------------------------------

// The fewest rows this notification can be drawn into: the padding, a single
// row of message, and the footer with its gap when shown.
func (n Notification) MinHeight() int {
	rows := padY*2 + 1
	if n.Colors.Footer != "" {
		rows += footerGap + 1
	}
	return rows
}

// The number of rows this notification needs when its text is wrapped to
// innerWidth — the popup width less its border, which the caller adds back on.
func (n Notification) Height(innerWidth int) int {
	m := newNotifyModel(n)
	top, footer := m.sections(max(innerWidth-padX*2, 1))

	rows := padY*2 + lipgloss.Height(top)
	if footer != "" {
		rows += footerGap + lipgloss.Height(footer)
	}

	return rows
}

// Draw a notification full-screen in the current terminal — which, when
// relaunched inside tmux, is the popup sized from the config — and block until
// the user dismisses it or picks an option.
func RunNotifyTUI(n Notification) (NotifyResult, error) {
	prog := tea.NewProgram(newNotifyModel(n), tea.WithAltScreen())

	final, err := prog.Run()
	if err != nil {
		return NotifyResult{}, fmt.Errorf("running notification tui: %w", err)
	}

	model, ok := final.(notifyModel)
	if !ok {
		return NotifyResult{}, fmt.Errorf("unexpected final model %T", final)
	}

	return model.result, nil
}
