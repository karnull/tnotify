// pkg/panel.go

package pkg

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

//- Types ------------------------------------------------------------------------------------------

// PanelItem is one waiting notification as the side panel needs it: what to
// draw, and the id whoever is keeping it knows it by.
type PanelItem struct {
	ID           int
	Notification Notification
}

// PanelSource is how the side panel reaches the notifications it lists. They
// are calls rather than a slice so the panel can pick up notifications that
// arrive while it is open, and so it needs to know nothing about where they are
// kept or how an answer finds its way back.
type PanelSource struct {
	// Load reads the notifications as they stand now, oldest first. It is
	// called when the panel opens and on every poll after that.
	Load func() ([]PanelItem, error)

	// Answer delivers what the user picked and disposes of the notification.
	// A notification whose answer could not be delivered is left where it is.
	Answer func(id int, selected []string) error

	// Delete throws a notification away unanswered.
	Delete func(id int) error
}

// panelModel is the bubbletea model that lists every waiting notification as a
// box of its own and tracks which one the user is working on.
type panelModel struct {
	source PanelSource
	colors NotifyColors

	items []PanelItem

	// The state of the user's interaction with each notification, kept by id so
	// that a poll finding the same notifications again does not throw away a
	// half-typed answer or the options ticked so far.
	boxes map[int]*notifyModel

	// focus indexes items. entered is whether the focused notification has been
	// opened for answering, rather than merely highlighted.
	focus   int
	entered bool

	// Why a notification's answer did not arrive, by id, shown on the box until
	// the user moves off it or an answer gets through.
	failures map[int]string

	// Why the notifications could not be read at all, if they could not be.
	loadErr string

	// Bumped whenever the panel deals with a notification itself. A read that
	// was already in flight by then describes the world before it, and would
	// put back on screen what the user has just got rid of.
	revision int

	// jumping is whether the user is part-way through typing ":<number>" to go
	// straight to a notification, and jump is the number so far.
	jumping bool
	jump    string

	width  int
	height int
}

// Sent when it is time to look for notifications that have arrived, or been
// dealt with, since the last look.
type panelTickMsg struct{}

// Carries the notifications back from a read of wherever they are kept, which
// happens off the update loop so a locked store cannot wedge the panel.
type panelItemsMsg struct {
	items []PanelItem
	err   error

	// What the panel had already done to the notifications when the read was
	// asked for, so an answer that arrives out of date can be told apart.
	revision int
}

const (
	// Blank columns either side of a box's text, inside its border.
	boxPadX = 1

	// The border a box is drawn in costs a column on each side.
	boxBorder = 2

	// Blank rows between one box and the next, and between the last box and the
	// key hints along the bottom.
	boxGap  = 1
	hintGap = 1

	// The fewest columns the panel can be drawn into: a box's border and
	// padding either side, and a single column of text.
	minPanelWidth = boxBorder + boxPadX*2 + 1

	// How often the panel looks for notifications it has not got yet.
	panelPoll = time.Second

	warnMark = "⚠ "
)

//- Private Helpers --------------------------------------------------------------------------------

// Ask for the notifications as they now stand.
func (m panelModel) loadCmd() tea.Cmd {
	revision := m.revision

	return func() tea.Msg {
		items, err := m.source.Load()
		return panelItemsMsg{items: items, err: err, revision: revision}
	}
}

// Ask to be woken when it is worth looking for new notifications again.
func panelTickCmd() tea.Cmd {
	return tea.Tick(panelPoll, func(time.Time) tea.Msg { return panelTickMsg{} })
}

// The notification under the cursor and the state of the user's interaction
// with it, or false when the panel is empty.
func (m panelModel) focused() (PanelItem, *notifyModel, bool) {
	if m.focus < 0 || m.focus >= len(m.items) {
		return PanelItem{}, nil, false
	}

	item := m.items[m.focus]
	box := m.boxes[item.ID]
	if box == nil {
		return PanelItem{}, nil, false
	}

	return item, box, true
}

// Take the notifications as they now stand, leaving the user where they were.
func (m *panelModel) reload(items []PanelItem) {
	// Which notification the cursor is on has to be remembered by id rather
	// than by position: one answered in another pane shifts up everything
	// listed below it.
	focusedID := 0
	if item, _, ok := m.focused(); ok {
		focusedID = item.ID
	}

	// Carrying the old state across is what makes a poll invisible to someone
	// part-way through answering; state for notifications that have since gone
	// would otherwise pile up for as long as the panel is open.
	boxes := make(map[int]*notifyModel, len(items))
	failures := make(map[int]string, len(m.failures))

	for _, item := range items {
		if box, ok := m.boxes[item.ID]; ok {
			boxes[item.ID] = box
		} else {
			fresh := newNotifyModel(item.Notification)
			fresh.setActive(false)
			boxes[item.ID] = &fresh
		}

		if failure, ok := m.failures[item.ID]; ok {
			failures[item.ID] = failure
		}
	}

	m.items, m.boxes, m.failures = items, boxes, failures

	// Follow the notification the cursor was on. One that has gone hands the
	// cursor to whatever has moved up into its place.
	index := slices.IndexFunc(items, func(item PanelItem) bool { return item.ID == focusedID })
	if index < 0 {
		m.entered = false
		index = min(m.focus, len(items)-1)
	}
	m.focus = max(index, 0)
}

// Move the cursor by delta notifications, wrapping at either end.
func (m *panelModel) moveFocus(delta int) {
	if len(m.items) == 0 {
		return
	}

	// A failure has been read by the time the user moves off the box it is on.
	if item, _, ok := m.focused(); ok {
		delete(m.failures, item.ID)
	}

	m.focus = (m.focus + delta + len(m.items)) % len(m.items)
}

// Open the focused notification for answering. One with nothing to pick can
// only be read and thrown away, so it is not opened at all.
func (m *panelModel) enter() {
	item, box, ok := m.focused()
	if !ok || !item.Notification.interactive() {
		return
	}

	box.setActive(true)
	m.entered = true
}

// Go back to moving between notifications, leaving the one just closed under
// the cursor.
func (m *panelModel) leave() {
	if _, box, ok := m.focused(); ok {
		box.setActive(false)
	}
	m.entered = false
}

// Send the focused notification's answer back to where it came from, and let it
// go once it has arrived.
func (m *panelModel) confirm() {
	item, box, ok := m.focused()
	if !ok {
		return
	}

	// Nothing to report: an empty text box, or no options ticked.
	selected := box.selection()
	if len(selected) == 0 {
		return
	}

	m.leave()

	// An answer with nowhere to go leaves the notification where it is, so the
	// user can see why and decide whether to throw it away themselves.
	if err := m.source.Answer(item.ID, selected); err != nil {
		m.failures[item.ID] = err.Error()
		return
	}

	delete(m.failures, item.ID)
	m.revision++
}

// Throw the focused notification away unanswered.
func (m *panelModel) remove() {
	item, _, ok := m.focused()
	if !ok {
		return
	}

	if err := m.source.Delete(item.ID); err != nil {
		m.failures[item.ID] = err.Error()
		return
	}

	m.revision++
}

// Move the cursor to the notification whose number has been typed, which is the
// one the panel draws that number on. A number past the end lands on the last
// notification rather than nowhere.
func (m *panelModel) jumpTo() {
	number, err := strconv.Atoi(m.jump)
	if err != nil || number < 1 || len(m.items) == 0 {
		return
	}

	if item, _, ok := m.focused(); ok {
		delete(m.failures, item.ID)
	}

	m.focus = min(number, len(m.items)) - 1
}

// Handle a keypress while the user is typing the number of a notification to go
// to. Only digits get that far, so a mistyped key does not quietly land the
// cursor somewhere unintended.
func (m panelModel) updateJumping(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key := msg.String(); key {
	case "ctrl+c":
		return m, tea.Quit

	case "esc":
		m.jumping, m.jump = false, ""

	case "enter":
		m.jumpTo()
		m.jumping, m.jump = false, ""

	case "backspace":
		// Rubbing out the last digit backs out of jumping altogether, which is
		// where the ":" that started it went.
		if m.jump == "" {
			m.jumping = false
		} else {
			m.jump = m.jump[:len(m.jump)-1]
		}

	default:
		if len(key) == 1 && key[0] >= '0' && key[0] <= '9' {
			m.jump += key
		}
	}

	return m, nil
}

// Handle a keypress while the user is moving between notifications.
func (m panelModel) updateFocused(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.jumping {
		return m.updateJumping(msg)
	}

	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case ":":
		m.jumping, m.jump = true, ""

	case "up", "k":
		m.moveFocus(-1)

	case "down", "j":
		m.moveFocus(1)

	case "enter":
		m.enter()

	case "delete", "backspace":
		m.remove()
		return m, m.loadCmd()
	}

	return m, nil
}

// Handle a keypress while the user is working through one notification's
// options.
func (m panelModel) updateEntered(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	item, box, ok := m.focused()
	if !ok {
		m.entered = false
		return m, nil
	}

	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit

	case "esc":
		m.leave()
		return m, nil

	case "up":
		box.moveCursor(-1)
		return m, nil

	case "down":
		box.moveCursor(1)
		return m, nil

	case "enter":
		m.confirm()
		return m, m.loadCmd()
	}

	// While the cursor is on the text input every remaining key is typing, so
	// the single-letter shortcuts below have to stand aside — and so does
	// backspace, which would otherwise throw away the notification being
	// answered rather than the character just typed.
	if box.onInput() {
		var cmd tea.Cmd
		box.input, cmd = box.input.Update(msg)
		return m, cmd
	}

	switch msg.String() {
	case "k":
		box.moveCursor(-1)
	case "j":
		box.moveCursor(1)
	case " ":
		if item.Notification.Multiple && box.cursor < len(box.checked) {
			box.checked[box.cursor] = !box.checked[box.cursor]
		}
	}

	return m, nil
}

// The style trouble is reported in. The config has no colour of its own for it,
// so it borrows the heading's, which is the one picked to stand out from the
// message it sits against.
func (m panelModel) warnStyle(width int) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(m.colors.Head)).
		Bold(true).
		Width(width)
}

// The top of a box, with the number the panel knows the notification by set
// into the border, along with anything there is to say about the notification
// beside it. It is drawn here rather than left to lipgloss, which borders a
// block but will not write anything into one.
func (m panelModel) boxTop(number int, note string, style lipgloss.Style) string {
	border := lipgloss.RoundedBorder()

	label := fmt.Sprintf("%s %d ", border.Top, number)
	if note != "" {
		label += fmt.Sprintf("%s %s ", border.Top, note)
	}

	fill := max(m.width-2-lipgloss.Width(label), 0)

	return style.Render(border.TopLeft + label + strings.Repeat(border.Top, fill) + border.TopRight)
}

// Draw one notification as a box of its own, numbered, and bordered in the
// focus colour when it is the one under the cursor.
func (m panelModel) boxView(item PanelItem, box *notifyModel, number int, focused bool) string {
	textWidth := max(m.width-boxBorder-boxPadX*2, 1)

	// The footer belongs to the popup, which draws one notification at a time;
	// the panel carries a single set of hints along its own bottom instead.
	top, _ := box.sections(textWidth)

	// An answer that did not arrive is reported on the notification it was
	// meant for, rather than somewhere the eye has to go looking for it.
	if failure, ok := m.failures[item.ID]; ok {
		top = lipgloss.JoinVertical(lipgloss.Left, top, "", m.warnStyle(textWidth).Render(warnMark+failure))
	}

	colour := m.colors.Border

	// A notification nobody is waiting on any more is greyed out, but the
	// cursor still has to be findable, so focus wins over it.
	switch {
	case focused:
		colour = m.colors.Selection
	case item.Notification.Expired:
		colour = m.colors.Expired
	}
	style := lipgloss.NewStyle().Foreground(lipgloss.Color(colour))

	note := ""
	if item.Notification.Expired {
		note = "timeout"
	}

	// The top is drawn separately so the number can sit in it, so the rest of
	// the box is bordered on three sides and joined underneath.
	rest := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderTop(false).
		BorderForeground(lipgloss.Color(colour)).
		Padding(0, boxPadX).
		Width(m.width - boxBorder).
		Render(top)

	return m.boxTop(number, note, style) + "\n" + rest
}

// Every row of every box, one after another, along with where the focused box
// begins and how tall it is.
func (m panelModel) boxLines() (lines []string, start, span int) {
	for i, item := range m.items {
		box := m.boxes[item.ID]
		if box == nil {
			continue
		}

		if len(lines) > 0 {
			for range boxGap {
				lines = append(lines, "")
			}
		}

		// Numbered from 1, the way the panel is read out loud and the way
		// "clear" counts them on the command line.
		rendered := strings.Split(m.boxView(item, box, i+1, i == m.focus), "\n")
		if i == m.focus {
			start, span = len(lines), len(rendered)
		}

		lines = append(lines, rendered...)
	}

	return lines, start, span
}

// The stack of boxes as it should be drawn into room rows, scrolled far enough
// that the focused notification is on screen.
func (m panelModel) scrolled(room int) string {
	if len(m.items) == 0 {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color(m.colors.Message)).
			Width(m.width).
			Render("no notifications waiting")
	}

	lines, start, span := m.boxLines()

	// Scrolled no further than it takes to bring the focused box into view, so
	// that as much as possible of what surrounds it stays on screen. A box
	// taller than the panel shows its top, which is where its message is.
	offset := 0
	if start+span > room {
		offset = min(start+span-room, start)
	}
	offset = min(offset, max(len(lines)-room, 0))

	return strings.Join(lines[offset:min(offset+room, len(lines))], "\n")
}

// The keys worth offering, in the order they are given up as the panel narrows:
// the ones a user is least likely to be hunting for go first.
func (m panelModel) hints() []string {
	item, box, ok := m.focused()
	if !ok {
		return []string{"[q] quit"}
	}

	if !m.entered {
		hints := []string{"[:n] jump", "[↑↓ jk] move", "[del] clear"}

		// Only a notification with something to pick is worth opening; the rest
		// can be read and thrown away, and no more.
		if item.Notification.interactive() {
			hints = append(hints, "[enter] open")
		}

		return append(hints, "[q] quit")
	}

	hints := []string{}
	if box.rowCount() > 1 {
		// j/k would be swallowed by the text input, so they are only advertised
		// when there is no typing to be done.
		if item.Notification.Custom {
			hints = append(hints, "[↑↓] move")
		} else {
			hints = append(hints, "[↑↓ jk] move")
		}
	}
	// Backing out is the one key a user will try without being told, so it is
	// given up before the toggle that nothing else in the panel hints at.
	hints = append(hints, "[esc] back")
	if item.Notification.Multiple {
		hints = append(hints, "[space] toggle")
	}

	return append(hints, "[enter] send")
}

// The hints as they should be drawn at the given width, dropping the least
// wanted until what is left fits.
func (m panelModel) hintText(width int) string {
	hints := m.hints()

	for i := range hints {
		if line := strings.Join(hints[i:], "  "); lipgloss.Width(line) <= width {
			return line
		}
	}

	return ""
}

// Start the panel: read the notifications, and keep reading them.
func (m panelModel) Init() tea.Cmd {
	return tea.Batch(m.loadCmd(), panelTickCmd(), textinput.Blink)
}

// Handle a keypress, a resize, or a fresh look at the notifications.
func (m panelModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case panelTickMsg:
		return m, tea.Batch(m.loadCmd(), panelTickCmd())

	case panelItemsMsg:
		// Asked for before the user dealt with something, and answered after:
		// acting on it would put the notification back.
		if msg.revision < m.revision {
			return m, nil
		}

		if msg.err != nil {
			// Printing it would land on top of the panel, so it is drawn as
			// part of it and the last good list is left up.
			m.loadErr = msg.err.Error()
			return m, nil
		}

		m.loadErr = ""
		m.reload(msg.items)
		return m, nil

	case tea.KeyMsg:
		if m.entered {
			return m.updateEntered(msg)
		}
		return m.updateFocused(msg)
	}

	return m, nil
}

// Draw the notifications down the pane, keeping the key hints against the
// bottom.
func (m panelModel) View() string {
	// Nothing sensible to draw until tmux has told us how big the pane is.
	if m.width < minPanelWidth || m.height < 1 {
		return ""
	}

	head := ""
	if m.loadErr != "" {
		head = m.warnStyle(m.width).Render(warnMark + m.loadErr)
	}

	// A number being typed takes the hints' place, since typing it blind is
	// worse than going without them — hidden footer or not.
	hint := ""
	switch {
	case m.jumping:
		hint = lipgloss.NewStyle().
			Foreground(lipgloss.Color(m.colors.Selection)).
			Width(m.width).
			Render(":" + m.jump)

	// A footer colour of "<hidden>" leaves the hints off altogether.
	case m.colors.Footer != "":
		hint = lipgloss.NewStyle().
			Foreground(lipgloss.Color(m.colors.Footer)).
			Width(m.width).
			Align(lipgloss.Center).
			Render(m.hintText(m.width))
	}

	// The rows each end costs, if it is drawn at all, counting the blank row
	// that separates it from the notifications.
	headRows, hintRows := 0, 0
	if head != "" {
		headRows = lipgloss.Height(head) + 1
	}
	if hint != "" {
		hintRows = hintGap + lipgloss.Height(hint)
	}

	room := max(m.height-headRows-hintRows, 1)
	body := m.scrolled(room)
	drawn := lipgloss.Height(body)

	content := body
	if head != "" {
		content = head + "\n\n" + body
	}
	if hint != "" {
		// Fewer notifications than the pane has room for leave slack; the hints
		// stay against the bottom either way. gap counts blank rows, so it
		// takes one more newline to draw.
		gap := max(room-drawn, 0) + hintGap
		content += strings.Repeat("\n", gap+1) + hint
	}

	return lipgloss.NewStyle().Width(m.width).Height(m.height).Render(content)
}

//- Public Calls -----------------------------------------------------------------------------------

// Draw every waiting notification as a box in the current terminal — which,
// when relaunched by tnotify, is the pane split off the side of the window —
// and block until the user closes the panel.
//
// Notifications are moved between with the arrow keys or j/k and thrown away
// with delete. Enter opens the one under the cursor, moving those same keys
// onto its options, where enter sends the answer back to the pane the
// notification came from and escape backs out again.
func RunPanelTUI(colors NotifyColors, source PanelSource) error {
	model := panelModel{
		source:   source,
		colors:   colors,
		boxes:    map[int]*notifyModel{},
		failures: map[int]string{},
	}

	if _, err := tea.NewProgram(model, tea.WithAltScreen()).Run(); err != nil {
		return fmt.Errorf("running notification panel: %w", err)
	}

	return nil
}
