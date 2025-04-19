package ui

import (
	"fmt"
	"github.com/pterm/pterm"
)

func (d *Display) showSuccessInternal(message string) {
	if d.options.Quiet {
		return
	}

	pterm.Success.Println(message)
}

func (d *Display) showInfoInternal(message string) {
	if d.options.Quiet {
		return
	}

	pterm.Info.Println(message)
}

func (d *Display) showWarningInternal(message string) {
	if d.options.Quiet {
		return
	}

	pterm.Warning.Println(message)
}

func (d *Display) showErrorInternal(message string) {
	if d.options.Quiet {
		return
	}

	pterm.Error.Println(message)
}

func (d *Display) showHeaderInternal(title string) {
	if d.options.Quiet {
		return
	}

	fmt.Println()
	pterm.DefaultHeader.WithBackgroundStyle(pterm.NewStyle(pterm.BgBlue)).WithMargin(2).Println(title)
	fmt.Println()
}

func (s *SpinnerAdapter) UpdateText(text string) {
	s.display.UpdateSpinnerText(text)
}

func (s *SpinnerAdapter) Success(text string) {
	s.display.queueEvent(EventSpinnerSuccess, text, nil)
}

func (s *SpinnerAdapter) Fail(text string) {
	s.display.queueEvent(EventSpinnerFail, text, nil)
}

func (s *SpinnerAdapter) Warning(text string) {
	s.display.queueEvent(EventSpinnerWarning, text, nil)
}

func (s *SpinnerAdapter) Info(text string) {
	s.display.queueEvent(EventSpinnerStop, text, nil)
}
