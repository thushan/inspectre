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

	// Create new progress bar
	bar, _ := pterm.DefaultProgressbar.
		WithTotal(total).
		WithTitle(title).
		WithRemoveWhenDone(true).
		Start()

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
