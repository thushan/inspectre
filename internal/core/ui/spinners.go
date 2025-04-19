package ui

import "github.com/pterm/pterm"

func (d *Display) startSpinnerInternal(text string) {
	if d.options.Quiet {
		return
	}

	d.spinnerMu.Lock()
	defer d.spinnerMu.Unlock()

	// Stop existing spinner if needed
	if d.spinner != nil {
		d.spinner.Stop()
	}

	// Create new spinner
	spinner, _ := pterm.DefaultSpinner.WithText(text).Start()
	d.spinner = spinner
}

func (d *Display) updateSpinnerTextInternal(text string) {
	if d.options.Quiet {
		return
	}

	d.spinnerMu.Lock()
	defer d.spinnerMu.Unlock()

	if d.spinner != nil {
		d.spinner.UpdateText(text)
	}
}

func (d *Display) stopSpinnerInternal(text string) {
	if d.options.Quiet {
		return
	}

	d.spinnerMu.Lock()
	defer d.spinnerMu.Unlock()

	if d.spinner != nil {
		d.spinner.Stop()
		d.spinner = nil
	}
}

func (d *Display) successSpinnerInternal(text string) {
	if d.options.Quiet {
		return
	}

	d.spinnerMu.Lock()
	defer d.spinnerMu.Unlock()

	if d.spinner != nil {
		d.spinner.Success(text)
		d.spinner = nil
	}
}

func (d *Display) failSpinnerInternal(text string) {
	if d.options.Quiet {
		return
	}

	d.spinnerMu.Lock()
	defer d.spinnerMu.Unlock()

	if d.spinner != nil {
		d.spinner.Fail(text)
		d.spinner = nil
	}
}

func (d *Display) warningSpinnerInternal(text string) {
	if d.options.Quiet {
		return
	}

	d.spinnerMu.Lock()
	defer d.spinnerMu.Unlock()

	if d.spinner != nil {
		d.spinner.Warning(text)
		d.spinner = nil
	}
}
