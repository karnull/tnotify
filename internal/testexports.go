// internal/testexports.go

package internal

// The tests live in test/ at the project root, which puts them in a package
// of their own and out of reach of everything below. This file is the one
// door through: the names stay unexported where they are written, and are
// reached from outside under exported ones here.

import (
	"github.com/charmbracelet/lipgloss"
)

//- Constants --------------------------------------------------------------------------------------

const (
	AuthorEnvVar  = authorEnvVar
	HiddenSetting = colorHidden
	OverlayBorder = overlayBorder
	HelpMinWidth  = helpMinWidth
	HelpMaxWidth  = helpMaxWidth
)

//- Types ------------------------------------------------------------------------------------------

type (
	StoredNotification = storedNotification
	ClearRequest       = clearRequest
)

//- Functions --------------------------------------------------------------------------------------

var (
	DefaultAuthor          = defaultAuthor
	ParentName             = parentName
	ScriptArg              = scriptArg
	NotifyColors           = notifyColors
	FitOverlay             = fitOverlay
	OverlayBounds          = overlayBounds
	PlaceOverlay           = placeOverlay
	ParseLocation          = parseLocation
	WrapText               = wrapText
	ParseNotify            = parseNotify
	ShowMode               = showMode
	ParseClear             = parseClear
	SelectForClearing      = selectForClearing
	AllNotifications       = allNotifications
	RememberNotification   = rememberNotification
	LastNotification       = lastNotification
	NotificationByID       = notificationByID
	AnswerNotification     = answerNotification
	ForgetNotification     = forgetNotification
	ForgetNotifications    = forgetNotifications
	StoredNotificationArgs = storedNotificationArgs
	DecodeResult           = decodeResult
	EncodeResult           = encodeResult
)

func RenderHelpAt(renderer *lipgloss.Renderer, width int) string {
	return renderHelpAt(renderer, width)
}

//- Test Hooks -------------------------------------------------------------------------------------

// StubAnnounceCount catches the counts the store would publish to tmux, so
// a test never reaches the session it is being run in. The returned
// function puts the real announcer back.
//
// The dedupe is process-wide, so it starts over both ways round: a later
// test would otherwise see nothing published for a count an earlier one
// used.
func StubAnnounceCount(announce func(waiting int) error) func() {
	announceCount, publishedCount = announce, -1

	return func() { announceCount, publishedCount = tmuxAnnounceCount, -1 }
}
