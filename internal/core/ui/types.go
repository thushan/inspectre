package ui

import (
	"context"
	"github.com/pterm/pterm"
	"github.com/thushan/inspectre/internal/core/logging"
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

	// Logger for display operations
	logger *logging.Logger
}

// SpinnerAdapter adapts the internal display to the SpinnerProvider interface
type SpinnerAdapter struct {
	display *Display
}

// UpdateText updates the spinner text
func (s *SpinnerAdapter) UpdateText(text string) {
	s.display.UpdateSpinnerText(text)
}

// Success marks the spinner as successful with the given text
func (s *SpinnerAdapter) Success(text string) {
	s.display.queueEvent(EventSpinnerSuccess, text, nil)
}

// Fail marks the spinner as failed with the given text
func (s *SpinnerAdapter) Fail(text string) {
	s.display.queueEvent(EventSpinnerFail, text, nil)
}

// Warning marks the spinner with a warning with the given text
func (s *SpinnerAdapter) Warning(text string) {
	s.display.queueEvent(EventSpinnerWarning, text, nil)
}

// Info stops the spinner and displays the given text as info
func (s *SpinnerAdapter) Info(text string) {
	s.display.queueEvent(EventSpinnerStop, text, nil)
}
