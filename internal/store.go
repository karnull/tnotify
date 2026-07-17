// internal/store.go

package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
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
	if err != nil || !changed {
		return err
	}

	return writeStore(path, store)
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

// Keep an ignored notification, and mark the pane it came from.
func rememberNotification(n storedNotification) error {
	return withStore(func(store *storeFile) (bool, error) {
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

		// A pane that has gone is not there to be retitled, and complaining that
		// it cannot be found says nothing the orphaning did not already say.
		if !dealt.Orphaned {
			waiting, _ := paneState(*store, dealt.Pane.ID)
			markPane(dealt.Pane.ID, dealt.Pane.Title, waiting)
		}

		return true, nil
	})
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
