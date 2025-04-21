package ui

import (
	"fmt"
	"github.com/pterm/pterm"
)

func (d *Display) showSuccessInternal(message string) {
	if d.options.Quiet {
		return
	}

	// Ensure spinner is stopped before displaying a message
	d.ensureSpinnerStopped()

	pterm.Success.Println(message)
}

func (d *Display) showInfoInternal(message string) {
	if d.options.Quiet {
		return
	}

	// Ensure spinner is stopped before displaying a message
	d.ensureSpinnerStopped()

	pterm.Info.Println(message)
}

func (d *Display) showWarningInternal(message string) {
	if d.options.Quiet {
		return
	}

	// Ensure spinner is stopped before displaying a message
	d.ensureSpinnerStopped()

	pterm.Warning.Println(message)
}

func (d *Display) showErrorInternal(message string) {
	if d.options.Quiet {
		return
	}

	// Ensure spinner is stopped before displaying a message
	d.ensureSpinnerStopped()

	pterm.Error.Println(message)
}

func (d *Display) showHeaderInternal(title string) {
	if d.options.Quiet {
		return
	}

	// Ensure spinner is stopped before displaying a header
	d.ensureSpinnerStopped()

	fmt.Println()

	// Use a dedicated header style with proper spacing
	header := pterm.DefaultHeader.
		WithBackgroundStyle(pterm.NewStyle(pterm.BgBlue)).
		WithMargin(0)

	header.Println(title)

	fmt.Println()
}

// ensureSpinnerStopped is a helper to make sure spinner is stopped before showing messages
func (d *Display) ensureSpinnerStopped() {
	d.spinnerMu.Lock()
	if d.spinner != nil {
		d.spinner.Stop()
		d.spinner = nil
	}
	d.spinnerMu.Unlock()
}
