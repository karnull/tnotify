// internal/overlay.go

package internal

import (
	"fmt"
	"strings"

	"github.com/karnull/tnotify/pkg"
)

// The tmux popup border costs a row/column on each side of the content.
const overlayBorder = 2

const locationUsage = "use <top|middle|bottom>-<left|center|right>"

//- Private Helpers --------------------------------------------------------------------------------

// Hold a value within a range.
func clamp(value, low, high int) int {
	return min(max(value, low), high)
}

// Split an overlay location ("top-left", "middle-center", ...) into its
// vertical and horizontal halves, either of which may be left out.
func parseLocation(location string) (vertical, horizontal string, err error) {
	vertical, horizontal = "middle", "center"
	verticalSet, horizontalSet := false, false

	// The halves may be given in either order, but naming one axis twice
	// ("middle-top") is an error rather than a silent pick, since it usually
	// means the axes were written the wrong way round.
	for part := range strings.SplitSeq(strings.ToLower(strings.TrimSpace(location)), "-") {
		switch part {
		case "top", "middle", "bottom":
			if verticalSet {
				return "", "", fmt.Errorf("invalid overlay location %q: %q and %q both set the vertical position, %s",
					location, vertical, part, locationUsage)
			}
			vertical, verticalSet = part, true

		case "left", "right":
			if horizontalSet {
				return "", "", fmt.Errorf("invalid overlay location %q: %q and %q both set the horizontal position, %s",
					location, horizontal, part, locationUsage)
			}
			horizontal, horizontalSet = part, true

		case "center", "centre", "":
			// Centred on whichever axis the other half doesn't name.
		default:
			return "", "", fmt.Errorf("invalid overlay location %q: %q is not a position, %s", location, part, locationUsage)
		}
	}

	return vertical, horizontal, nil
}

// Work out the popup sizes a config allows, in cells and including the border.
// heightFloor is the shortest popup the notification can be drawn in.
func overlayBounds(cfg Config, termWidth, termHeight, heightFloor int) (minWidth, maxWidth, minHeight, maxHeight int) {
	// A zero min_/max_ in the config means "unbounded", which resolves to the
	// smallest size the TUI can draw into and the size of the terminal.
	maxWidth = termWidth
	if cfg.Overlay.MaxWidth > 0 {
		maxWidth = min(cfg.Overlay.MaxWidth, termWidth)
	}
	minWidth = clamp(cfg.Overlay.MinWidth, pkg.MinWidth+overlayBorder, maxWidth)

	maxHeight = termHeight
	if cfg.Overlay.MaxHeight > 0 {
		maxHeight = min(cfg.Overlay.MaxHeight, termHeight)
	}
	minHeight = clamp(cfg.Overlay.MinHeight, heightFloor, maxHeight)

	return minWidth, maxWidth, minHeight, maxHeight
}

// Choose the smallest popup the message fits in, spending width before height.
// All sizes include the border.
func fitOverlay(n pkg.Notification, minWidth, maxWidth, minHeight, maxHeight int) (width, height int) {
	// The shortest the message can be shown, given it may not be wider than
	// max_width — and never shorter than min_height, so vertical room that has
	// been asked for is not paid for in width the message doesn't need.
	target := max(minHeight, n.Height(maxWidth-overlayBorder)+overlayBorder)

	// Wrapping at a wider width never needs more rows, so "reaches the target"
	// only ever goes from false to true as the popup widens, and the narrowest
	// width that does can be binary searched for. Falls out at max_width when
	// nothing reaches it.
	low, high := minWidth, maxWidth
	for low < high {
		mid := (low + high) / 2
		if n.Height(mid-overlayBorder)+overlayBorder <= target {
			high = mid
		} else {
			low = mid + 1
		}
	}
	width = low

	height = clamp(n.Height(width-overlayBorder)+overlayBorder, minHeight, maxHeight)

	return width, height
}

// Place a popup of the given size within a terminal of the given size. tmux
// takes X as the popup's left column and Y as the row just past its bottom.
func placeOverlay(location string, width, height, termWidth, termHeight int) (pkg.Overlay, error) {
	vertical, horizontal, err := parseLocation(location)
	if err != nil {
		return pkg.Overlay{}, err
	}

	overlay := pkg.Overlay{Width: width, Height: height}

	switch horizontal {
	case "left":
		overlay.X = 0
	case "right":
		overlay.X = termWidth - width
	default:
		overlay.X = (termWidth - width) / 2
	}

	switch vertical {
	case "top":
		overlay.Y = height
	case "bottom":
		overlay.Y = termHeight
	default:
		overlay.Y = (termHeight-height)/2 + height
	}

	return overlay, nil
}

// Size and place an overlay around a notification.
func notifyOverlay(cfg Config, n pkg.Notification) (pkg.Overlay, error) {
	termWidth, termHeight, err := pkg.TmuxClientSize()
	if err != nil {
		return pkg.Overlay{}, err
	}

	minWidth, maxWidth, minHeight, maxHeight := overlayBounds(cfg, termWidth, termHeight, n.MinHeight()+overlayBorder)
	width, height := fitOverlay(n, minWidth, maxWidth, minHeight, maxHeight)

	return placeOverlay(cfg.Overlay.Location, width, height, termWidth, termHeight)
}

// Size and place an overlay with no message to measure: the widest it is
// allowed, and half the terminal's height.
func plainOverlay(cfg Config) (pkg.Overlay, error) {
	termWidth, termHeight, err := pkg.TmuxClientSize()
	if err != nil {
		return pkg.Overlay{}, err
	}

	_, maxWidth, minHeight, maxHeight := overlayBounds(cfg, termWidth, termHeight, pkg.Notification{}.MinHeight()+overlayBorder)
	height := clamp(termHeight/2, minHeight, maxHeight)

	return placeOverlay(cfg.Overlay.Location, maxWidth, height, termWidth, termHeight)
}
