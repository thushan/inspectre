package version

import (
	"fmt"
	"log"
)

var (
	Name        = "inspectre"
	Authors     = "Thushan Fernando"
	Description = "A code analysis tool to inspect repositories"
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
	githubUri := GithubHomeText
	latestUri := Version
	padLatest := fmt.Sprintf("%*s", 40-len(Version), "")

	vlog.Println(`╔──────────────────────────────────────────────────────────────────────────╗
│  ██╗███╗   ██╗███████╗██████╗ ███████╗ ██████╗████████╗██████╗ ███████╗  │
│  ██║████╗  ██║██╔════╝██╔══██╗██╔════╝██╔════╝╚══██╔══╝██╔══██╗██╔════╝  │
│  ██║██╔██╗ ██║███████╗██████╔╝█████╗  ██║        ██║   ██████╔╝█████╗    │
│  ██║██║╚██╗██║╚════██║██╔═══╝ ██╔══╝  ██║        ██║   ██╔══██╗██╔══╝    │
│  ██║██║ ╚████║███████║██║     ███████╗╚██████╗   ██║   ██║  ██║███████╗  │
│  ╚═╝╚═╝  ╚═══╝╚══════╝╚═╝     ╚══════╝ ╚═════╝   ╚═╝   ╚═╝  ╚═╝╚══════╝  │`)
	vlog.Println("│ ", githubUri, padLatest, latestUri, " │")
	vlog.Println(`╚──────────────────────────────────────────────────────────────────────────╝`)
	if extendedInfo {
		vlog.Println(" Commit:", Commit)
		vlog.Println("  Built:", Date)
		vlog.Println("  Using:", User)
		vlog.Println("")
	}
}
