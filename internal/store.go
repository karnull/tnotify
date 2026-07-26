// internal/store.go

package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/karnull/tnotify/pkg"
)

const (
	storeDirName  = "tnotify"
	storeFileName = "notifications.json"
	storeLockName = "notifications.lock"
)

// The layout version written into the store, so a later change to the format
// can tell an old file apart from a new one rather than misread it.
const storeVersion = 1

// The title a pane is given while notifications are waiting on it, followed by
// how many. A pane wearing this is one tnotify still expects to answer into.
const paneMarker = "tnotify"

// The tmux global environment variable the number of waiting notifications is
// published in, for a status line to draw as "#{TNOTIFY_COUNT}".
const countEnvVar = "TNOTIFY_COUNT"

// The count this process last told tmux about, so reading the store over and
// over — as the side panel does — does not spend a tmux call re-announcing a
// number that has not moved. It starts unset rather than at zero, which is what
// makes the first store access of every process publish: a tmux server that has
// been restarted has forgotten the count, and the store has not.
var (
	publishedMu    sync.Mutex
	publishedCount = -1

	// How the count reaches tmux, as a variable so that a test can watch it go
	// without a tmux server to send it to — and, run inside one, without
	// writing the test's own count over the session's.
	announceCount = tmuxAnnounceCount
)

// storedNotification is a notification the user ignored, kept in full so it can
// be shown again exactly as it first arrived and answered afterwards.
type storedNotification struct {
	ID int `json:"id"`

	// When the notification was sent, not when it was ignored.
	Sent time.Time `json:"sent"`

	// Author is who the notification is shown as being from, which --author or
	// TNOTIFY_AUTHOR may have set. TrueAuthor is whatever actually ran tnotify,
	// regardless of what it asked to be called.
	Author     string `json:"author"`
	TrueAuthor string `json:"true_author"`

	Head string `json:"head,omitempty"`
	Body string `json:"body"`

	Options     []string `json:"options,omitempty"`
	Interactive bool     `json:"interactive,omitempty"`
	Custom      bool     `json:"custom,omitempty"`
	Multiple    bool     `json:"multiple,omitempty"`

	// The pane the notification was sent from, so an answer given later has
	// somewhere to go back to.
	Pane pkg.PaneInfo `json:"pane"`

	// Where a process still waiting on this notification wants the answer
	// written, and which process that is. An answer given long after the popup
	// closed goes back to whoever asked for it rather than into their pane.
	Reply  string `json:"reply,omitempty"`
	Waiter int    `json:"waiter,omitempty"`

	// Set once the caller stopped waiting for an answer. What it asked is kept,
	// but there is nobody left to answer, so it becomes a plain message.
	Expired bool `json:"expired,omitempty"`

	// Set once that pane has closed. An orphaned notification can still be read
	// and answered; only its answer has nowhere left to go.
	Orphaned bool `json:"orphaned,omitempty"`
}

// storeFile is the on-disk shape of the notification store.
type storeFile struct {
	Version       int                  `json:"version"`
	NextID        int                  `json:"next_id"`
	Notifications []storedNotification `json:"notifications"`
}

//- Private Helpers --------------------------------------------------------------------------------

// Return the directory tnotify keeps its state in, honouring XDG_STATE_HOME and
// otherwise ~/.local/state/tnotify.
func stateDir() (string, error) {
	if dir := strings.TrimSpace(os.Getenv("XDG_STATE_HOME")); dir != "" {
		return filepath.Join(dir, storeDirName), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("determining home directory: %w", err)
	}

	return filepath.Join(home, ".local", "state", storeDirName), nil
}

// Take the store's lock, held until the returned function is called. The lock
// lives in a file of its own so that it survives the store being replaced by a
// rename, and the kernel drops it if tnotify dies still holding it.
func lockStore(dir string) (unlock func(), err error) {
	file, err := os.OpenFile(filepath.Join(dir, storeLockName), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("opening notification lock: %w", err)
	}

	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		file.Close()
		return nil, fmt.Errorf("locking notification store: %w", err)
	}

	return func() {
		syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		file.Close()
	}, nil
}

// Read the store, treating a file that is not there yet as an empty one.
func readStore(path string) (storeFile, error) {
	store := storeFile{Version: storeVersion, NextID: 1}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return store, nil
		}
		return store, fmt.Errorf("reading notification store: %w", err)
	}

	if err := json.Unmarshal(data, &store); err != nil {
		return store, fmt.Errorf("parsing notification store: %w", err)
	}

	// Ids are handed out from here, so a missing or nonsensical counter would
	// otherwise start reusing them.
	if store.NextID < 1 {
		store.NextID = 1
	}

	return store, nil
}

// Write the store out in one piece.
func writeStore(path string, store storeFile) error {
	store.Version = storeVersion

	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding notification store: %w", err)
	}
	data = append(data, '\n')

	// Written beside the store and renamed over it, so anyone reading part-way
	// through sees either the old file or the new one, never half of either.
	temp, err := os.CreateTemp(filepath.Dir(path), ".notifications-*.json")
	if err != nil {
		return fmt.Errorf("writing notification store: %w", err)
	}
	name := temp.Name()

	if _, err := temp.Write(data); err != nil {
		temp.Close()
		os.Remove(name)
		return fmt.Errorf("writing notification store: %w", err)
	}
	if err := temp.Close(); err != nil {
		os.Remove(name)
		return fmt.Errorf("writing notification store: %w", err)
	}

	if err := os.Rename(name, path); err != nil {
		os.Remove(name)
		return fmt.Errorf("writing notification store: %w", err)
	}

	return nil
}

// Hand the count to tmux's global environment. Outside tmux there is nobody to
// hand it to, which is not a failure: the count is only ever there to be read
// off a status line.
func tmuxAnnounceCount(waiting int) error {
	if !pkg.InsideTmux() {
		return nil
	}
	return pkg.TmuxSetGlobalEnv(countEnvVar, strconv.Itoa(waiting))
}

// Publish how many notifications are waiting, so a status line reading
// "#{TNOTIFY_COUNT}" can show it. A count that will not go across is not worth
// interrupting the command over: nothing about the notification depends on it,
// and the next store access publishes again anyway.
func publishCount(waiting int) {
	publishedMu.Lock()
	defer publishedMu.Unlock()

	if waiting == publishedCount {
		return
	}

	if err := announceCount(waiting); err != nil {
		return
	}
	publishedCount = waiting
}

// Run fn against the stored notifications, writing the result back when fn
// reports it changed something. The store is locked for the whole call, so two
// tnotify processes cannot lose each other's changes.
func withStore(fn func(store *storeFile) (changed bool, err error)) error {
	dir, err := stateDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating state directory: %w", err)
	}

	unlock, err := lockStore(dir)
	if err != nil {
		return err
	}
	defer unlock()

	path := filepath.Join(dir, storeFileName)

	store, err := readStore(path)
	if err != nil {
		return err
	}

	changed, err := fn(&store)
	if err != nil {
		return err
	}

	if changed {
		if err := writeStore(path, store); err != nil {
			return err
		}
	}

	// Published from under the lock, and off a read as much as a write, so the
	// number tmux is holding is the store as it stands rather than as some other
	// tnotify left it a moment later.
	publishCount(len(store.Notifications))

	return nil
}

// Count the notifications still waiting on a pane, and recover the title that
// pane had before tnotify first renamed it.
func paneState(store storeFile, paneID string) (waiting int, title string) {
	for _, n := range store.Notifications {
		if n.Pane.ID == paneID {
			waiting++
			title = n.Pane.Title
		}
	}
	return waiting, title
}

// Retitle a pane to show how many notifications are waiting on it, or back to
// the title it started with once none are. A pane that will not take a new
// title is not worth failing over: the notification is stored either way.
func markPane(id, original string, waiting int) {
	if id == "" {
		return
	}

	title := original
	if waiting > 0 {
		title = fmt.Sprintf("%s:%d", paneMarker, waiting)
	}

	if err := pkg.TmuxRenamePane(id, title); err != nil {
		reportError(fmt.Errorf("renaming pane: %w", err))
	}
}

// Whether a pane title is one tnotify put there.
func markedPane(title string) bool {
	return strings.HasPrefix(title, paneMarker+":")
}

// Keep an ignored notification, mark the pane it came from, and report the id
// it was filed under so the caller can come back to it.
func rememberNotification(n storedNotification) (int, error) {
	var id int

	err := withStore(func(store *storeFile) (bool, error) {
		waiting, original := paneState(*store, n.Pane.ID)

		// Only the first notification on a pane sees the pane's own title; after
		// that the pane is already wearing tnotify's, so the original is carried
		// across from the notification that captured it.
		if waiting == 0 {
			original = n.Pane.Title
		}
		n.Pane.Title = original

		n.ID = store.NextID
		store.NextID++
		store.Notifications = append(store.Notifications, n)

		markPane(n.Pane.ID, original, waiting+1)

		id = n.ID
		return true, nil
	})

	return id, err
}

// Mark a notification as one nobody is waiting on any more, and forget where
// its caller was, since that caller has gone.
func expireNotification(id int) error {
	return withStore(func(store *storeFile) (bool, error) {
		index := slices.IndexFunc(store.Notifications, func(n storedNotification) bool {
			return n.ID == id
		})
		if index < 0 {
			return false, nil
		}

		store.Notifications[index].Expired = true
		store.Notifications[index].Reply = ""
		store.Notifications[index].Waiter = 0

		return true, nil
	})
}

// The most recently ignored notification still waiting to be answered.
func lastNotification() (storedNotification, bool, error) {
	var last storedNotification
	var found bool

	err := withStore(func(store *storeFile) (bool, error) {
		if count := len(store.Notifications); count > 0 {
			last, found = store.Notifications[count-1], true
		}
		return false, nil
	})

	return last, found, err
}

// Every notification still waiting to be answered, oldest first.
func allNotifications() ([]storedNotification, error) {
	var all []storedNotification

	err := withStore(func(store *storeFile) (bool, error) {
		// Cloned rather than handed out, since the slice belongs to a store
		// that stops being locked the moment this returns.
		all = slices.Clone(store.Notifications)
		return false, nil
	})

	return all, err
}

// The notification with the given id, if it is still waiting.
func notificationByID(id int) (storedNotification, bool, error) {
	var found storedNotification
	var ok bool

	err := withStore(func(store *storeFile) (bool, error) {
		index := slices.IndexFunc(store.Notifications, func(n storedNotification) bool {
			return n.ID == id
		})
		if index >= 0 {
			found, ok = store.Notifications[index], true
		}
		return false, nil
	})

	return found, ok, err
}

// Drop a notification that has been dealt with, and give its pane back the
// title it had if nothing else is waiting on it.
func forgetNotification(id int) error {
	return withStore(func(store *storeFile) (bool, error) {
		index := slices.IndexFunc(store.Notifications, func(n storedNotification) bool {
			return n.ID == id
		})
		if index < 0 {
			return false, nil
		}

		dealt := store.Notifications[index]
		store.Notifications = slices.Delete(store.Notifications, index, index+1)

		// Anything still waiting on this notification has to be let go, or it
		// would wait on an answer that is never coming. Releasing an already
		// answered one is harmless: its reply has been written.
		releaseWaiter(dealt, pkg.NotifyResult{Action: pkg.ActionClear})

		// A pane that has gone is not there to be retitled, and complaining that
		// it cannot be found says nothing the orphaning did not already say.
		if !dealt.Orphaned {
			waiting, _ := paneState(*store, dealt.Pane.ID)
			markPane(dealt.Pane.ID, dealt.Pane.Title, waiting)
		}

		return true, nil
	})
}

// Drop several notifications at once, and give each pane they came from back
// the title it had if nothing is left waiting on it. Ids that are not there are
// counted as already dealt with rather than as a failure.
func forgetNotifications(ids []int) (cleared int, err error) {
	err = withStore(func(store *storeFile) (bool, error) {
		going := make(map[int]bool, len(ids))
		for _, id := range ids {
			going[id] = true
		}

		// Which pane each is owed to, and the title to put back, have to be
		// read off before the notifications carrying them are deleted.
		titles := map[string]string{}
		for _, n := range store.Notifications {
			if !going[n.ID] {
				continue
			}
			// Anything still waiting on one of these has to be let go.
			releaseWaiter(n, pkg.NotifyResult{Action: pkg.ActionClear})

			if !n.Orphaned {
				titles[n.Pane.ID] = n.Pane.Title
			}
		}

		before := len(store.Notifications)
		store.Notifications = slices.DeleteFunc(store.Notifications, func(n storedNotification) bool {
			return going[n.ID]
		})
		cleared = before - len(store.Notifications)

		// Retitled once per pane rather than once per notification, so a pane
		// several were waiting on is not renamed over and over on the way down.
		for pane, title := range titles {
			waiting, _ := paneState(*store, pane)
			markPane(pane, title, waiting)
		}

		return cleared > 0, nil
	})

	return cleared, err
}

// Mark every notification whose pane has since closed, so it is not mistaken
// for one that can still be answered back to where it came from.
func reapOrphans() error {
	panes, err := pkg.TmuxPanes()
	if err != nil {
		return err
	}

	return withStore(func(store *storeFile) (bool, error) {
		changed := false

		for i, n := range store.Notifications {
			// Only whether the pane is still open, not what it is called: a
			// program in the pane can retitle it out from under tnotify, and
			// that is no reason to give up on the notification.
			_, open := panes[n.Pane.ID]

			if orphaned := !open; orphaned != n.Orphaned {
				store.Notifications[i].Orphaned = orphaned
				changed = true
			}
		}

		return changed, nil
	})
}
