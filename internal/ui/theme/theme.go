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

func init() {}

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
