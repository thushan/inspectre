package ui

import "github.com/pterm/pterm"

func (d *Display) startProgressInternal(total int, title string) {
	if d.options.Quiet {
		return
	}

	d.progressMu.Lock()
	defer d.progressMu.Unlock()

	// Stop existing progress bar if needed
	if d.progressBar != nil {
		d.progressBar.Stop()
	}

	// First make sure spinner is stopped to avoid conflicts
	d.spinnerMu.Lock()
	if d.spinner != nil {
		d.spinner.Stop()
		d.spinner = nil
	}
	d.spinnerMu.Unlock()

	// Create a clean progress bar configuration
	progressBar := pterm.DefaultProgressbar.
		WithTotal(total).
		WithTitle(title).
		WithRemoveWhenDone(true)

	// Create new progress bar
	bar, _ := progressBar.Start()
	d.progressBar = bar
}

func (d *Display) updateProgressInternal(increment int) {
	if d.options.Quiet {
		return
	}

	d.progressMu.Lock()
	defer d.progressMu.Unlock()

	if d.progressBar != nil {
		d.progressBar.Add(increment)
	}
}

func (d *Display) stopProgressInternal() {
	if d.options.Quiet {
		return
	}

	d.progressMu.Lock()
	defer d.progressMu.Unlock()

	if d.progressBar != nil {
		d.progressBar.Stop()
		d.progressBar = nil
	}
}
