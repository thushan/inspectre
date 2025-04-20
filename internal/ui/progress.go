package ui

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/pterm/pterm"
	"github.com/thushan/inspectre/internal/ui/theme"
)

type ProgressTracker struct {
	taskID      string
	spinner     *pterm.SpinnerPrinter
	progressBar *pterm.ProgressbarPrinter
	writer      io.Writer
	startTime   time.Time
	mu          sync.Mutex
	completed   bool
	failed      bool
}

func NewProgressTracker(taskID string, description string) *ProgressTracker {
	spinner, _ := pterm.DefaultSpinner.
		WithRemoveWhenDone(false).
		WithSequence(theme.SequenceIndexing...).
		WithText(fmt.Sprintf("%s: %s", theme.ColourTaskId(taskID), description)).
		Start()

	return &ProgressTracker{
		taskID:    taskID,
		spinner:   spinner,
		startTime: time.Now(),
	}
}

func (p *ProgressTracker) UpdateProgress(percentage float64, message string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.completed || p.failed {
		return
	}

	p.spinner.UpdateText(fmt.Sprintf("[%s] %s (%.0f%%)",
		theme.ColourTaskId(p.taskID),
		message,
		percentage))

	if p.progressBar != nil {
		p.progressBar.Current = int(percentage)
		// With pterm, you simply set Current and don't need to call Update separately
	}
}

func (p *ProgressTracker) StartProgressBar(title string, total int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.progressBar == nil {
		p.progressBar = pterm.DefaultProgressbar.
			WithTotal(total).
			WithTitle(title).
			WithRemoveWhenDone(false)

		_, err := p.progressBar.Start()
		if err != nil {
			// Just log the error, don't crash the application :(
			fmt.Printf("Failed to start progress bar: %v\n", err)
		}
	}
}

func (p *ProgressTracker) UpdateMessage(message string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.completed || p.failed {
		return
	}

	elapsedTime := time.Since(p.startTime).Round(time.Second)
	p.spinner.UpdateText(fmt.Sprintf("[%s] %s (%s)",
		theme.ColourTaskId(p.taskID),
		message,
		elapsedTime))
}

func (p *ProgressTracker) Complete(message string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.completed || p.failed {
		return
	}

	p.completed = true
	elapsedTime := time.Since(p.startTime).Round(time.Second)

	if p.progressBar != nil {
		p.progressBar.Current = p.progressBar.Total
		_, _ = p.progressBar.Stop()
	}

	p.spinner.Success(fmt.Sprintf("%s (%s)", message, elapsedTime))
}

func (p *ProgressTracker) Fail(message string, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.completed || p.failed {
		return
	}

	p.failed = true
	errorMessage := message
	if err != nil {
		errorMessage = fmt.Sprintf("%s: %v", message, err)
	}

	// Stop the progress bar if we have one
	if p.progressBar != nil {
		_, _ = p.progressBar.Stop()
	}

	p.spinner.Fail(errorMessage)
}

type ProgressManager struct {
	trackers map[string]*ProgressTracker
	mu       sync.RWMutex
}

func NewProgressManager() *ProgressManager {
	return &ProgressManager{
		trackers: make(map[string]*ProgressTracker),
	}
}

func (pm *ProgressManager) CreateTracker(taskID string, description string) *ProgressTracker {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	tracker := NewProgressTracker(taskID, description)
	pm.trackers[taskID] = tracker
	return tracker
}

func (pm *ProgressManager) GetTracker(taskID string) *ProgressTracker {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	return pm.trackers[taskID]
}

func (pm *ProgressManager) RemoveTracker(taskID string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	delete(pm.trackers, taskID)
}
