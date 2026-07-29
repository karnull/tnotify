// pkg/panel_test.go

package pkg

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

)

//- Private Helpers --------------------------------------------------------------------------------

// fakeSource stands in for the notification store, keeping what the panel asks
// of it so a test can check the panel asked for the right thing.
type fakeSource struct {
	items []PanelItem

	answered  map[int][]string
	deleted   []int
	answerErr error
}

// The three notifications the tests work with: one that can only be read, one
// that picks a single option, and one that ticks several and types its own.
func newFakeSource() *fakeSource {
	return &fakeSource{
		items: []PanelItem{
			{ID: 1, Notification: Notification{Author: "deploy.sh", Body: "staging is up"}},
			{ID: 2, Notification: Notification{Author: "claude", Body: "promote?", Options: []string{"yes", "no"}}},
			{ID: 3, Notification: Notification{
				Author:   "backup",
				Body:     "which volumes?",
				Options:  []string{"home", "srv"},
				Custom:   true,
				Multiple: true,
			}},
		},
		answered: map[int][]string{},
	}
}

// Drop a notification the way answering or clearing it would.
func (f *fakeSource) drop(id int) {
	f.items = slices.DeleteFunc(slices.Clone(f.items), func(item PanelItem) bool { return item.ID == id })
}

func (f *fakeSource) source() PanelSource {
	return PanelSource{
		// Cloned on the way out, as a store that has to unlock itself before
		// returning must: the panel holds on to what it is given.
		Load: func() ([]PanelItem, error) { return slices.Clone(f.items), nil },

		Answer: func(id int, selected []string) error {
			if f.answerErr != nil {
				return f.answerErr
			}
			f.answered[id] = selected
			f.drop(id)
			return nil
		},

		Delete: func(id int) error {
			f.deleted = append(f.deleted, id)
			f.drop(id)
			return nil
		},
	}
}

<<<<<<< HEAD:pkg/panel_test.go
<<<<<<<< HEAD:pkg/pkg_panel_test.go
// A panel of the given size, already showing the source's notifications.
func newTestPanel(t *testing.T, f *fakeSource, width, height int) tea.Model {
	t.Helper()
========
// A panel of the given size, drawing arrival times the given way.
func newPanel(f *fakeSource, clock PanelClock, width, height int) tea.Model {
	var model tea.Model = panelModel{
		source:   f.source(),
		clock:    clock,
		boxes:    map[int]*notifyModel{},
		failures: map[int]string{},
	}

	model, _ = model.Update(tea.WindowSizeMsg{Width: width, Height: height})
	return model
}

// A panel of the given size, already showing the source's notifications.
func newTestPanel(t *testing.T, f *fakeSource, width, height int) tea.Model {
	t.Helper()
	return press(t, newPanel(f, PanelClock{}, width, height))
}
>>>>>>>> 5d0a602 (fixup! feat: list waiting notifications in a side panel):pkg/panel_test.go
=======
// A panel of the given size, drawing arrival times the given way.
func newPanel(f *fakeSource, clock pkg.PanelClock, width, height int) tea.Model {
	var model tea.Model = pkg.NewPanelModel(f.source(), clock)

	model, _ = model.Update(tea.WindowSizeMsg{Width: width, Height: height})
	return model
}

// A panel of the given size, already showing the source's notifications.
func newTestPanel(t *testing.T, f *fakeSource, width, height int) tea.Model {
	t.Helper()
	return press(t, newPanel(f, pkg.PanelClock{}, width, height))
}
>>>>>>> 9e8613d (feat: stamp each notification with the time it arrived):test/pkg_panel_test.go

// The one notification a source holds, arrived at the given time and drawn into
// a panel of the given width with the default clock and date.
func newClockPanel(t *testing.T, sent time.Time, width int) string {
	t.Helper()

<<<<<<< HEAD:pkg/panel_test.go
<<<<<<<< HEAD:pkg/pkg_panel_test.go
	model, _ = model.Update(tea.WindowSizeMsg{Width: width, Height: height})
	return press(t, model)
========
	f := &fakeSource{
		items:    []PanelItem{{ID: 1, Notification: Notification{Author: "deploy.sh", Body: "up"}, Sent: sent}},
		answered: map[int][]string{},
	}

	clock := PanelClock{Time: "15:04", Date: "02/01/2006"}
	return press(t, newPanel(f, clock, width, 20)).View()
>>>>>>>> 5d0a602 (fixup! feat: list waiting notifications in a side panel):pkg/panel_test.go
=======
	f := &fakeSource{
		items:    []pkg.PanelItem{{ID: 1, Notification: pkg.Notification{Author: "deploy.sh", Body: "up"}, Sent: sent}},
		answered: map[int][]string{},
	}

	clock := pkg.PanelClock{Time: "15:04", Date: "02/01/2006"}
	return press(t, newPanel(f, clock, width, 20)).View()
>>>>>>> 9e8613d (feat: stamp each notification with the time it arrived):test/pkg_panel_test.go
}

// Turn a key's name into the message bubbletea would deliver for it.
func keyMsg(key string) tea.KeyMsg {
	switch key {
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "delete":
		return tea.KeyMsg{Type: tea.KeyDelete}
	case "backspace":
		return tea.KeyMsg{Type: tea.KeyBackspace}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
}

// Drive the panel through a run of keypresses. Commands the panel asks for are
// run and their answers fed back, since a key that reloads the list is only
// half of what the user sees happen.
func press(t *testing.T, model tea.Model, keys ...string) tea.Model {
	t.Helper()

	drain := func(cmd tea.Cmd) {
		if cmd == nil {
			return
		}
		if msg, ok := cmd().(panelItemsMsg); ok {
			model, _ = model.Update(msg)
		}
	}

	// The panel starts empty and fills itself in, so a run of no keys at all is
	// still worth driving: it is how a test gets the notifications on screen.
	drain(model.(panelModel).loadCmd())

	for _, key := range keys {
		var cmd tea.Cmd
		model, cmd = model.Update(keyMsg(key))

		// A keypress that lands in the text input hands back the cursor's blink
		// timer, which there is no reason for a test to sit and wait out.
		// Everything else the panel asks for is a reload.
		if _, box, ok := model.(panelModel).focused(); ok && box.onInput() {
			continue
		}

		drain(cmd)
	}

	return model
}

// The panel's state, for a test to make assertions against.
func state(t *testing.T, model tea.Model) panelModel {
	t.Helper()

	m, ok := model.(panelModel)
	if !ok {
		t.Fatalf("model is %T, want panelModel", model)
	}

	return m
}

// The id of the notification under the cursor.
func focusedID(t *testing.T, model tea.Model) int {
	t.Helper()

	item, _, ok := state(t, model).focused()
	if !ok {
		t.Fatal("panel has nothing focused, want a notification")
	}

	return item.ID
}

//- Tests ------------------------------------------------------------------------------------------

// The newest notification is not the one waiting longest, so the cursor starts
// at the top of the list and wraps rather than stopping at either end.
func TestPanelFocusStartsAtTheTopAndWraps(t *testing.T) {
	f := newFakeSource()
	panel := newTestPanel(t, f, 40, 40)

	if got := focusedID(t, panel); got != 1 {
		t.Errorf("focused id = %d, want the first notification (1)", got)
	}

	if got := focusedID(t, press(t, panel, "k")); got != 3 {
		t.Errorf("focused id after k = %d, want the last notification (3)", got)
	}

	if got := focusedID(t, press(t, panel, "j", "j", "j")); got != 1 {
		t.Errorf("focused id after wrapping right round = %d, want 1", got)
	}
}

// Entering a notification hands it the same keys the list was using, and
// leaving it hands them back without moving the cursor off it.
func TestEnteringMovesTheKeysOntoTheOptions(t *testing.T) {
	f := newFakeSource()
	panel := press(t, newTestPanel(t, f, 40, 40), "j", "enter")

	m := state(t, panel)
	if !m.entered {
		t.Fatal("panel did not enter the notification on enter")
	}

	_, box, _ := state(t, press(t, panel, "j")).focused()
	if box.cursor != 1 {
		t.Errorf("option cursor after j = %d, want 1", box.cursor)
	}

	// The list must not have moved underneath it: j went to the options.
	if got := focusedID(t, press(t, panel, "j")); got != 2 {
		t.Errorf("focused id after j inside the notification = %d, want it left on 2", got)
	}

	left := state(t, press(t, panel, "j", "esc"))
	if left.entered {
		t.Error("panel stayed entered after esc")
	}
	if got := focusedID(t, press(t, panel, "j", "esc")); got != 2 {
		t.Errorf("focused id after esc = %d, want the notification left in focus (2)", got)
	}
}

// The whole point of the panel: an answer given here goes back to whoever asked
// for it, and the notification stops waiting.
func TestEnterSendsTheAnswerAndDropsTheNotification(t *testing.T) {
	f := newFakeSource()
	panel := press(t, newTestPanel(t, f, 40, 40), "j", "enter", "j", "enter")

	if got, want := f.answered[2], []string{"no"}; !slices.Equal(got, want) {
		t.Errorf("answered = %q, want %q", got, want)
	}

	m := state(t, panel)
	if m.entered {
		t.Error("panel stayed entered after sending the answer")
	}
	if len(m.items) != 2 {
		t.Errorf("panel lists %d notifications, want the answered one gone (2)", len(m.items))
	}
}

// An answer that could not be delivered is worth more than the notification is:
// it stays put, saying why, rather than disappearing along with the answer.
func TestAFailedAnswerKeepsTheNotification(t *testing.T) {
	f := newFakeSource()
	f.answerErr = fmt.Errorf("pane %%3 has closed")

	panel := press(t, newTestPanel(t, f, 40, 40), "j", "enter", "enter")

	m := state(t, panel)
	if len(m.items) != 3 {
		t.Errorf("panel lists %d notifications, want the undelivered one kept (3)", len(m.items))
	}
	if m.failures[2] == "" {
		t.Error("panel recorded no failure against the notification")
	}
	if !strings.Contains(panel.View(), "has closed") {
		t.Error("panel does not say why the answer did not arrive")
	}

	// Having read it, the user moves on; the warning has done its job.
	if failures := state(t, press(t, panel, "j")).failures; len(failures) != 0 {
		t.Errorf("failures after moving off = %v, want them cleared", failures)
	}
}

// Enter on a highlighted option has to answer with it even where several could
// have been ticked. Taking it as "nothing picked" leaves the key doing nothing
// at all, which reads as the notification being stuck.
func TestEnterTakesTheHighlightedOptionWhenNoneAreTicked(t *testing.T) {
	f := newFakeSource()

	// The third notification is the one that ticks several with space.
	panel := press(t, newTestPanel(t, f, 40, 40), "j", "j", "enter", "j", "enter")

	if got, want := f.answered[3], []string{"srv"}; !slices.Equal(got, want) {
		t.Errorf("answered = %q, want the highlighted option %q", got, want)
	}
	if len(state(t, panel).items) != 2 {
		t.Error("the answered notification is still waiting")
	}
}

// Ticking options must still be what enter answers with when the user has
// bothered to tick any.
func TestEnterTakesEveryTickedOption(t *testing.T) {
	f := newFakeSource()
	press(t, newTestPanel(t, f, 40, 40), "j", "j", "enter", " ", "j", " ", "enter")

	if got, want := f.answered[3], []string{"home", "srv"}; !slices.Equal(got, want) {
		t.Errorf("answered = %q, want %q", got, want)
	}
}

// A read that was already in flight when the user answered describes the world
// before they did, and must not put the notification back on screen.
func TestAStalePollDoesNotResurrectAnAnsweredNotification(t *testing.T) {
	f := newFakeSource()
	panel := newTestPanel(t, f, 40, 40)

	// What a poll issued just before the answer would carry back with it.
	stale := panelItemsMsg{items: slices.Clone(f.items), revision: state(t, panel).revision}

	panel = press(t, panel, "j", "enter", "enter")
	if len(state(t, panel).items) != 2 {
		t.Fatalf("panel lists %d notifications, want the answered one gone", len(state(t, panel).items))
	}

	panel, _ = panel.Update(stale)

	if got := len(state(t, panel).items); got != 2 {
		t.Errorf("panel lists %d notifications after a stale poll, want 2", got)
	}
}

// A notification the user is done with can be thrown away without answering it.
func TestDeleteThrowsTheFocusedNotificationAway(t *testing.T) {
	f := newFakeSource()
	panel := press(t, newTestPanel(t, f, 40, 40), "j", "delete")

	if !slices.Equal(f.deleted, []int{2}) {
		t.Errorf("deleted = %v, want the focused notification (2)", f.deleted)
	}

	m := state(t, panel)
	if len(m.items) != 2 {
		t.Fatalf("panel lists %d notifications, want 2", len(m.items))
	}
	if got := focusedID(t, panel); got != 3 {
		t.Errorf("focused id = %d, want the notification that moved up (3)", got)
	}
}

// Backspace clears a notification in the list, so a user typing their own
// answer must be able to correct it without losing the notification instead.
func TestBackspaceWhileTypingEditsTheAnswer(t *testing.T) {
	f := newFakeSource()

	// Down onto the third notification, in, past both options onto the text
	// input, and type into it.
	panel := press(t, newTestPanel(t, f, 40, 40),
		"j", "j", "enter", "j", "j", "p", "r", "o", "d", "backspace")

	_, box, _ := state(t, panel).focused()
	if got := box.input.Value(); got != "pro" {
		t.Errorf("typed answer = %q, want %q", got, "pro")
	}
	if len(f.deleted) != 0 {
		t.Errorf("backspace deleted %v while the user was typing", f.deleted)
	}
}

// The panel re-reads the notifications while it is open, which must not be
// something the user answering one can feel happening.
func TestAPollLeavesAHalfGivenAnswerAlone(t *testing.T) {
	f := newFakeSource()

	panel := press(t, newTestPanel(t, f, 40, 40), "j", "j", "enter", " ", "j", "j", "s", "r", "v")
	panel, _ = panel.Update(panelItemsMsg{items: f.items, revision: 0})

	m := state(t, panel)
	if !m.entered {
		t.Fatal("a poll dropped the user out of the notification they were answering")
	}

	_, box, _ := m.focused()
	if got := box.input.Value(); got != "srv" {
		t.Errorf("typed answer after a poll = %q, want %q", got, "srv")
	}
	if !box.checked[0] {
		t.Error("a poll unticked the option the user had ticked")
	}
}

// Notifications answered in another pane shift the list underneath the cursor,
// which must stay on the notification the user was reading rather than on
// whatever has taken its position.
func TestFocusFollowsTheNotificationItIsOn(t *testing.T) {
	f := newFakeSource()
	panel := press(t, newTestPanel(t, f, 40, 40), "j")

	// The one above it is answered from the pane it was sent to.
	f.drop(1)
	panel, _ = panel.Update(panelItemsMsg{items: f.items, revision: 0})

	if got := focusedID(t, panel); got != 2 {
		t.Errorf("focused id = %d, want the notification the cursor was on (2)", got)
	}
	if got := state(t, panel).focus; got != 0 {
		t.Errorf("focus index = %d, want it to have moved up with the list (0)", got)
	}
}

// A notification the user was answering can be dealt with elsewhere, and the
// panel has to give up on it rather than answer a notification that has gone.
func TestAFocusedNotificationTakenAwayIsLeftBehind(t *testing.T) {
	f := newFakeSource()
	panel := press(t, newTestPanel(t, f, 40, 40), "j", "enter")

	f.drop(2)
	panel, _ = panel.Update(panelItemsMsg{items: f.items, revision: 0})

	m := state(t, panel)
	if m.entered {
		t.Error("panel is still answering a notification that has gone")
	}
	if got := focusedID(t, panel); got != 3 {
		t.Errorf("focused id = %d, want whatever moved into its place (3)", got)
	}
}

// A notification with nothing to pick has no options to move between, so
// entering it would trap the keys somewhere with nothing to do.
func TestANotificationWithNothingToPickCannotBeEntered(t *testing.T) {
	f := newFakeSource()
	panel := press(t, newTestPanel(t, f, 40, 40), "enter")

	if state(t, panel).entered {
		t.Error("panel entered a notification that has nothing to pick")
	}
}

// More notifications than the pane is tall is the case the panel exists for, so
// the one being worked on has to be the one on screen.
func TestScrollingKeepsTheFocusedNotificationOnScreen(t *testing.T) {
	f := newFakeSource()

	// Room for roughly one notification at a time.
	panel := newTestPanel(t, f, 40, 10)

	if view := panel.View(); !strings.Contains(view, "deploy.sh") {
		t.Errorf("panel does not show the focused notification:\n%s", view)
	}

	bottom := press(t, panel, "k")
	if view := bottom.View(); !strings.Contains(view, "which volumes?") {
		t.Errorf("panel did not scroll to the focused notification:\n%s", view)
	}

	back := press(t, bottom, "j")
	if view := back.View(); !strings.Contains(view, "staging is up") {
		t.Errorf("panel did not scroll back to the focused notification:\n%s", view)
	}
}

// The panel is drawn into a pane of its own, so it must fill it exactly: short
// and the pane keeps whatever was there before, long and the hints scroll off.
func TestPanelFillsThePaneItIsDrawnIn(t *testing.T) {
	f := newFakeSource()

	for _, height := range []int{10, 24, 60} {
		panel := newTestPanel(t, f, 40, height)

		for _, keys := range [][]string{{}, {"k"}, {"j", "enter"}} {
			view := press(t, panel, keys...).View()

			if got := strings.Count(view, "\n") + 1; got != height {
				t.Errorf("panel of height %d after %v drew %d rows", height, keys, got)
			}
		}
	}
}

// A box's top border is drawn by hand so the number can sit in it, which puts
// its width beyond lipgloss's reach: too narrow and the border steps in from
// the sides, too wide and every row below it wraps.
func TestEveryRowIsExactlyThePaneWidth(t *testing.T) {
	f := newFakeSource()

	for _, width := range []int{20, 34, 41, 80} {
		view := newTestPanel(t, f, width, 40).View()

		for i, line := range strings.Split(view, "\n") {
			if got := lipgloss.Width(line); got != width {
				t.Errorf("panel of width %d drew row %d at %d columns: %q", width, i, got, line)
			}
		}
	}
}

// The numbers are how a notification is named, both to jump to it here and to
// clear it from the command line, so they count the whole list from 1.
func TestBoxesAreNumberedFromOne(t *testing.T) {
	f := newFakeSource()
	view := newTestPanel(t, f, 40, 40).View()

	for number := 1; number <= 3; number++ {
		if label := fmt.Sprintf("─ %d ─", number); !strings.Contains(view, label) {
			t.Errorf("panel does not number a notification %d:\n%s", number, view)
		}
	}
}

<<<<<<< HEAD:pkg/panel_test.go
<<<<<<<< HEAD:pkg/pkg_panel_test.go
========
=======
>>>>>>> 9e8613d (feat: stamp each notification with the time it arrived):test/pkg_panel_test.go
// A notification from today needs no date on it: a bare clock is read as today
// already, and the border has little enough room as it is.
func TestTodayIsDrawnAsAClockAlone(t *testing.T) {
	// Anchored to today so the panel agrees about which day "today" is,
	// whenever the test happens to be run.
	now := time.Now()
	sent := time.Date(now.Year(), now.Month(), now.Day(), 14, 32, 0, 0, time.Local)

	view := newClockPanel(t, sent, 40)

	if !strings.Contains(view, "─ 14:32 ─╮") {
		t.Errorf("panel does not draw the arrival time against the far corner:\n%s", view)
	}
	if strings.Contains(view, "/") {
		t.Errorf("panel dates a notification that arrived today:\n%s", view)
	}

	// The number stays where it was: the two of them are the ends of the same
	// border, not a pair at one end of it.
	if !strings.Contains(view, "╭─ 1 ─") {
		t.Errorf("panel moved the number off the near corner:\n%s", view)
	}
}

// A clock on its own says nothing about which day it means, so anything from
// before today carries the date it arrived on.
func TestOlderNotificationsCarryTheirDate(t *testing.T) {
	sent := time.Now().AddDate(0, 0, -2)
	view := newClockPanel(t, sent, 40)

	if want := sent.Format("15:04 02/01/2006"); !strings.Contains(view, want) {
		t.Errorf("panel does not draw %q on a notification from two days ago:\n%s", want, view)
	}
}

// A notification nothing was recorded for is drawn without a time rather than
// as the beginning of the epoch.
func TestANotificationWithNoTimeIsDrawnWithout(t *testing.T) {
	view := newClockPanel(t, time.Time{}, 40)

	if strings.Contains(view, "00:00") || strings.Contains(view, "1970") {
		t.Errorf("panel drew a time for a notification that has none:\n%s", view)
	}
}

// The border is only so wide, and the number is what a notification is named
// by, so the time is what gives way as the panel narrows.
func TestTheTimeGivesWayBeforeTheNumber(t *testing.T) {
	sent := time.Now().AddDate(0, 0, -2)

<<<<<<< HEAD:pkg/panel_test.go
	for _, width := range []int{40, 28, 20, 14, minPanelWidth} {
=======
	for _, width := range []int{40, 28, 20, 14, pkg.MinPanelWidth} {
>>>>>>> 9e8613d (feat: stamp each notification with the time it arrived):test/pkg_panel_test.go
		view := newClockPanel(t, sent, width)

		if !strings.Contains(view, "╭─ 1") {
			t.Errorf("panel of width %d dropped the number:\n%s", width, view)
		}

		// Whatever it decided to draw, the border still has to be exactly as
		// wide as the pane it is drawn in.
		for i, line := range strings.Split(view, "\n") {
			if got := lipgloss.Width(line); got != width {
				t.Errorf("panel of width %d drew row %d at %d columns: %q", width, i, got, line)
			}
		}
	}

	// Wide enough for the clock but not the date, which is the first thing to
	// go: 20 columns leaves 14 between the corners, and " 15:04 02/01/2006 "
	// alone wants 18 of them.
	if view := newClockPanel(t, sent, 20); !strings.Contains(view, sent.Format("15:04")) {
		t.Errorf("panel of width 20 dropped the clock as well as the date:\n%s", view)
	}
	if view := newClockPanel(t, sent, 20); strings.Contains(view, "/") {
		t.Errorf("panel of width 20 kept the date:\n%s", view)
	}
}

<<<<<<< HEAD:pkg/panel_test.go
>>>>>>>> 5d0a602 (fixup! feat: list waiting notifications in a side panel):pkg/panel_test.go
=======
>>>>>>> 9e8613d (feat: stamp each notification with the time it arrived):test/pkg_panel_test.go
// Going straight to a notification is the point of numbering them.
func TestJumpingToANotificationByNumber(t *testing.T) {
	f := newFakeSource()
	panel := newTestPanel(t, f, 40, 40)

	if got := focusedID(t, press(t, panel, ":", "3", "enter")); got != 3 {
		t.Errorf("focused id after :3 = %d, want 3", got)
	}

	// The number is shown as it is typed, so it is not being entered blind.
	if view := press(t, panel, ":", "2").View(); !strings.Contains(view, ":2") {
		t.Errorf("panel does not show the number being typed:\n%s", view)
	}
}

// A number past the end is a slip, not a reason to leave the cursor nowhere.
func TestJumpingPastTheEndLandsOnTheLast(t *testing.T) {
	f := newFakeSource()

	if got := focusedID(t, press(t, newTestPanel(t, f, 40, 40), ":", "9", "enter")); got != 3 {
		t.Errorf("focused id after :9 = %d, want the last notification (3)", got)
	}
}

// Escape has to get out of a half-typed number without moving the cursor, and
// backspace has to rub the number out rather than clear a notification.
func TestAJumpCanBeBackedOutOf(t *testing.T) {
	f := newFakeSource()
	panel := press(t, newTestPanel(t, f, 40, 40), "j")

	if got := focusedID(t, press(t, panel, ":", "3", "esc")); got != 2 {
		t.Errorf("focused id after an escaped jump = %d, want it left alone (2)", got)
	}

	// Rubbed back to nothing, then a digit that is no longer part of a jump.
	backed := press(t, panel, ":", "3", "backspace", "backspace", "1")
	if got := focusedID(t, backed); got != 2 {
		t.Errorf("focused id after backing out = %d, want it left alone (2)", got)
	}
	if state(t, backed).jumping {
		t.Error("panel is still jumping after the number was rubbed out")
	}
	if len(f.deleted) != 0 {
		t.Errorf("backspace deleted %v while a number was being typed", f.deleted)
	}
}
<<<<<<<< HEAD:pkg/pkg_panel_test.go
========

// A notification whose caller stopped waiting is drawn as the plain message it
// has become: named as timed out, with nothing left to pick.
func TestExpiredNotificationIsPlainAndLabelled(t *testing.T) {
	f := newFakeSource()
	f.items[1].Notification.Expired = true

	panel := newTestPanel(t, f, 40, 40)
	view := panel.View()

	if !strings.Contains(view, "─ 2 ─ timeout ─") {
		t.Errorf("panel does not mark the expired notification:\n%s", view)
	}
	// Its options belonged to an answer nobody is waiting for any more.
	if strings.Contains(view, "1. yes") {
		t.Errorf("expired notification still offers its options:\n%s", view)
	}
	// The one below it is untouched.
	if !strings.Contains(view, "1. home") {
		t.Errorf("a live notification lost its options:\n%s", view)
	}
}

// There is nothing to answer on an expired notification, so entering it would
// trap the keys somewhere with nothing to do. It can only be thrown away.
func TestExpiredNotificationCannotBeEntered(t *testing.T) {
	f := newFakeSource()
	f.items[1].Notification.Expired = true

	panel := press(t, newTestPanel(t, f, 40, 40), "j", "enter")
	if state(t, panel).entered {
		t.Error("panel entered a notification nobody is waiting on")
	}

	if hints := strings.Join(state(t, panel).hints(), " "); strings.Contains(hints, "open") {
		t.Errorf("hints still offer to open it: %q", hints)
	}

	// Deleting it is the one thing left.
	press(t, panel, "delete")
	if !slices.Contains(f.deleted, 2) {
		t.Errorf("deleted = %v, want the expired notification (2)", f.deleted)
	}
}
>>>>>>>> 5d0a602 (fixup! feat: list waiting notifications in a side panel):pkg/panel_test.go
