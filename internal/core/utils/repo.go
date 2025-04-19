package utils

import "strings"

// extractRepoName extracts a repository name from its URL
func ExtractRepoName(url string) string {
	// Remove trailing .git if present
	if strings.HasSuffix(url, ".git") {
		url = url[:len(url)-4]
	}

	// Extract the last component of the path
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		name := parts[len(parts)-1]
		if name != "" {
			return name
		}
	}

	// Fallback to a generic name
	return "repository"
}

// GuessRepoType tries to determine the repository type from the URL
func GuessRepoType(url string) string {
	if strings.Contains(url, "github.com") {
		return "github"
	} else if strings.Contains(url, "gitlab.com") {
		return "gitlab"
	} else if strings.Contains(url, "bitbucket.org") {
		return "bitbucket"
	}
	return "git" // default type
}
