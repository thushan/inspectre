package version

import (
	"fmt"
	"github.com/thushan/inspectre/internal/core/ui/theme"
	"log"
)

var (
	Name        = "inspectre"
	Authors     = "Thushan Fernando"
	Description = "A code analyser tool to inspect repositories"
	Version     = "v2.0.25"
	Commit      = "none"
	Date        = "nowish"
	User        = "local"
)

const (
	GithubHomeText  = "github.com/thushan/inspectre"
	GithubHomeUri   = "https://github.com/thushan/inspectre"
	GithubLatestUri = "https://github.com/thushan/inspectre/releases/latest"
)

func PrintVersionInfo(extendedInfo bool, vlog *log.Logger) {
	githubUri := theme.Hyperlink(GithubHomeUri, GithubHomeText)
	latestUri := theme.Hyperlink(GithubLatestUri, Version)
	padLatest := fmt.Sprintf("%*s", 40-len(Version), "")

	vlog.Println(theme.ColourSplash(`╔──────────────────────────────────────────────────────────────────────────╗
│  ██╗███╗   ██╗███████╗██████╗ ███████╗ ██████╗████████╗██████╗ ███████╗  │
│  ██║████╗  ██║██╔════╝██╔══██╗██╔════╝██╔════╝╚══██╔══╝██╔══██╗██╔════╝  │
│  ██║██╔██╗ ██║███████╗██████╔╝█████╗  ██║        ██║   ██████╔╝█████╗    │
│  ██║██║╚██╗██║╚════██║██╔═══╝ ██╔══╝  ██║        ██║   ██╔══██╗██╔══╝    │
│  ██║██║ ╚████║███████║██║     ███████╗╚██████╗   ██║   ██║  ██║███████╗  │
│  ╚═╝╚═╝  ╚═══╝╚══════╝╚═╝     ╚══════╝ ╚═════╝   ╚═╝   ╚═╝  ╚═╝╚══════╝  │`))
	vlog.Println(theme.ColourSplash("│ "), theme.StyleUrl(githubUri), padLatest, theme.ColourVersion(latestUri), theme.ColourSplash(" │"))
	vlog.Println(theme.ColourSplash(`╚──────────────────────────────────────────────────────────────────────────╝`))
	if extendedInfo {
		vlog.Println(" Commit:", Commit)
		vlog.Println("  Built:", Date)
		vlog.Println("  Using:", User)
		vlog.Println("")
	}
}
