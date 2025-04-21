package ui

import (
	"fmt"
	"github.com/pterm/pterm"
	"os"
	"strings"
	"time"
)

// startSpinnerInternal starts a new spinner with better isolation
func (d *Display) startSpinnerInternal(text string) {
	if d.options.Quiet {
		return
	}

	d.spinnerMu.Lock()
	defer d.spinnerMu.Unlock()

	// Stop existing spinner if needed
	if d.spinner != nil {
		// Use the Stop method which properly terminates the spinner
		d.spinner.Stop()

		// Sleep briefly to ensure terminal state is reset
		time.Sleep(50 * time.Millisecond)

		d.spinner = nil
	}

	// Add a newline before starting the spinner for better separation
	fmt.Fprintln(os.Stdout)

	// Create a custom spinner configuration to avoid conflicts
	// Using different sequence for better visibility
	spinner := pterm.DefaultSpinner.
		WithRemoveWhenDone(true).
		WithDelay(80 * time.Millisecond) // Slightly slower for better visibility

	// Clear message style to avoid conflicts with text coloring
	spinner.MessageStyle = pterm.NewStyle()

	// Start new spinner
	s, _ := spinner.WithText(text).Start()
	d.spinner = s
}

// updateSpinnerTextInternal ensures spinner text updates are clean
func (d *Display) updateSpinnerTextInternal(text string) {
	if d.options.Quiet {
		return
	}

	d.spinnerMu.Lock()
	defer d.spinnerMu.Unlock()

	if d.spinner != nil {
		// Update the text with proper handling
		d.spinner.UpdateText(text)
	}
}

// stopSpinnerInternal ensures proper cleanup of spinner resources
func (d *Display) stopSpinnerInternal(text string) {
	if d.options.Quiet {
		return
	}

	d.spinnerMu.Lock()
	defer d.spinnerMu.Unlock()

	if d.spinner != nil {
		// If a message is provided, update the text before stopping
		if text != "" {
			d.spinner.UpdateText(text)
		}

		// Clear the last line to avoid conflicts with upcoming output
		clearLine()

		// Stop the spinner and sleep briefly to ensure terminal state is reset
		d.spinner.Stop()
		time.Sleep(50 * time.Millisecond)
		d.spinner = nil

		// If text was provided, print it on a clean line
		if text != "" {
			fmt.Println(text)
		}

		// Add a newline for separation
		fmt.Println()
	}
}

// successSpinnerInternal shows a successful completion with proper formatting
func (d *Display) successSpinnerInternal(text string) {
	if d.options.Quiet {
		return
	}

	d.spinnerMu.Lock()
	defer d.spinnerMu.Unlock()

	if d.spinner != nil {
		// Clear current spinner line for clean output
		clearLine()

		if text != "" {
			d.spinner.UpdateText(text)
		}

		// Stop the spinner with success indication
		d.spinner.Success()
		d.spinner = nil

		// Add a newline for better separation
		fmt.Println()
	} else {
		// If no spinner exists, just show a success message
		d.showSuccessInternal(text)
	}
}

// failSpinnerInternal shows a failure completion with proper formatting
func (d *Display) failSpinnerInternal(text string) {
	if d.options.Quiet {
		return
	}

	d.spinnerMu.Lock()
	defer d.spinnerMu.Unlock()

	if d.spinner != nil {
		// Clear current spinner line for clean output
		clearLine()

		if text != "" {
			d.spinner.UpdateText(text)
		}

		// Stop the spinner with fail indication
		d.spinner.Fail(text)
		d.spinner = nil

		// Add a newline for better separation
		fmt.Println()
	} else {
		// If no spinner exists, just show an error message
		d.showErrorInternal(text)
	}
}

func (d *Display) warningSpinnerInternal(text string) {
	if d.options.Quiet {
		return
	}

	d.spinnerMu.Lock()
	defer d.spinnerMu.Unlock()

	if d.spinner != nil {
		// Clear current spinner line for clean output
		clearLine()

		if text != "" {
			d.spinner.UpdateText(text)
		}

		// Stop the spinner with warning indication
		d.spinner.Warning(text)
		d.spinner = nil

		// Add a newline for better separation
		fmt.Println()
	} else {
		// If no spinner exists, just show a warning message
		d.showWarningInternal(text)
	}
}

// clearLine clears the current line in the terminal
func clearLine() {
	// Escape sequence to clear the current line and move cursor to beginning
	fmt.Fprint(os.Stdout, "\r"+strings.Repeat(" ", 80)+"\r")
}
