package ui

import (
	"context"
	"github.com/pterm/pterm"
	"sync"
	"time"
)

// DisplayOptions represents display options
type DisplayOptions struct {
	NoColor bool
	Format  string // "table", "json", "text"
	Quiet   bool
}

// UIEvent represents an event that needs to be displayed
type UIEvent struct {
	Type      string
	Message   string
	Data      interface{}
	Timestamp time.Time
}

// Display represents a UI display manager
type Display struct {
	options     DisplayOptions
	spinner     *pterm.SpinnerPrinter
	progressBar *pterm.ProgressbarPrinter

	// Event handling
	eventChan chan UIEvent
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup

	// Thread safety for UI components
	spinnerMu  sync.Mutex
	progressMu sync.Mutex
	closed     bool
	closedMu   sync.RWMutex
}

// SpinnerAdapter adapts the internal display to the SpinnerProvider interface
type SpinnerAdapter struct {
	display *Display
}
