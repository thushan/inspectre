package theme

import (
	"github.com/pterm/pterm"
)

var (
	WithContextGlyph = "└─"

	SequenceIndexing = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	SequenceSmashing = []string{"◰", "◳", "◲", "◱"}
	SequenceFinalise = []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}
	SequenceInternet = []string{"🌍", "🌎", "🌏"}
	SequenceTimeSoon = []string{"🕐", "🕑", "🕒", "🕓", "🕔", "🕕", "🕖", "🕗", "🕘", "🕙", "🕚", "🕛"}
	SequenceTimeLong = []string{"🕐", "🕜", "🕑", "🕝", "🕒", "🕞", "🕓", "🕟", "🕔", "🕠", "🕕", "🕡", "🕖", "🕢", "🕗", "🕣", "🕘", "🕤", "🕙", "🕥", "🕚", "🕦", "🕛", "🕧"}

	SequenceSmashingAlt = []string{"⬒", "⬔", "⬓", "⬕"}
)

func init() {

	skippingPrefix := pterm.Warning.Prefix
	skippingPrefix.Text = "SKIP"

	verbosePrefix := pterm.Info.Prefix
	verbosePrefix.Text = "VERBOSE"

}

func ColourSplash(message ...any) string {
	return pterm.LightGreen(message...)
}
func StyleUrl(message ...any) string {
	return pterm.LightBlue(message...)
}
func Hyperlink(uri string, text string) string {
	return "\x1b]8;;" + uri + "\x07" + text + "\x1b]8;;\x07" + "\u001b[0m"
}

func ColourVersion(message ...any) string {
	return pterm.LightYellow(message...)
}
func ColourTaskId(message ...any) string {
	return pterm.Gray(message...)
}
func ColourRepository(message ...any) string {
	return pterm.LightGreen(message...)
}
func ColourWorkDir(message ...any) string {
	return pterm.LightMagenta(message...)
}
func ColourAnalyser(message ...any) string {
	return pterm.LightBlue(message...)
}

// ColourStatus returns a colored representation of a status
func ColourStatus(status string) string {
	switch status {
	case "Completed":
		return pterm.Green(status)
	case "Running":
		return pterm.Blue(status)
	case "Created", "Queued":
		return pterm.Cyan(status)
	case "Failed":
		return pterm.Red(status)
	case "Cancelled":
		return pterm.Yellow(status)
	default:
		return status
	}
}
