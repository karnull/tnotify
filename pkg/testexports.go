// pkg/testexports.go

package pkg

// The tests live in test/ at the project root, which puts them in a package
// of their own and out of reach of everything below. This file is the one
// door through: the models stay unexported and so do their fields, and are
// reached from outside under exported names here.

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

//- Types ------------------------------------------------------------------------------------------

type (
	NotifyModel   = notifyModel
	PanelModel    = panelModel
	PanelItemsMsg = panelItemsMsg
)

//- Functions --------------------------------------------------------------------------------------

var (
	SplitFlags = splitFlags
)

// NewPanelModel is a panel reading from the given source, before it has
// been given a size or anything to list.
func NewPanelModel(source PanelSource) panelModel {
	return panelModel{
		source:   source,
		boxes:    map[int]*notifyModel{},
		failures: map[int]string{},
	}
}

// NewPanelItemsMsg is what a read of the notifications carries back, as
// one issued at the given revision would.
func NewPanelItemsMsg(items []PanelItem, revision int) panelItemsMsg {
	return panelItemsMsg{items: items, revision: revision}
}

//- Panel State ------------------------------------------------------------------------------------

func (m panelModel) LoadCmd() tea.Cmd                         { return m.loadCmd() }
func (m panelModel) Focused() (PanelItem, *NotifyModel, bool) { return m.focused() }
func (m panelModel) Hints() []string                          { return m.hints() }

func (m panelModel) Items() []PanelItem       { return m.items }
func (m panelModel) Failures() map[int]string { return m.failures }
func (m panelModel) Focus() int               { return m.focus }
func (m panelModel) Entered() bool            { return m.entered }
func (m panelModel) Jumping() bool            { return m.jumping }
func (m panelModel) Revision() int            { return m.revision }

//- Notification State -----------------------------------------------------------------------------

func (m notifyModel) OnInput() bool { return m.onInput() }

func (m notifyModel) Cursor() int            { return m.cursor }
func (m notifyModel) Checked() []bool        { return m.checked }
func (m notifyModel) Input() textinput.Model { return m.input }
