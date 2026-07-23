// internal/testexports.go

package internal

// The tests live in test/ at the project root, which puts them in a package
// of their own and out of reach of everything below. This file is the one
// door through: the names stay unexported where they are written, and are
// reached from outside under exported ones here.

//- Constants --------------------------------------------------------------------------------------

const (
	AuthorEnvVar  = authorEnvVar
	HiddenSetting = colorHidden
	OverlayBorder = overlayBorder
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
