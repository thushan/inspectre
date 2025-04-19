package repository

import (
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"strings"
)

// getAuthMethod determines the appropriate authentication method
func (m *Manager) getAuthMethod(repo *Repository) (transport.AuthMethod, error) {
	switch strings.ToLower(repo.Type) {
	case "github", "gitlab":
		if repo.Auth.Token != "" {
			return &http.BasicAuth{
				Username: "x-oauth-basic", // For GitHub, the username doesn't matter
				Password: repo.Auth.Token,
			}, nil
		} else if repo.Auth.Username != "" && repo.Auth.Password != "" {
			return &http.BasicAuth{
				Username: repo.Auth.Username,
				Password: repo.Auth.Password,
			}, nil
		}
		// This is for public repositories
		return nil, nil
	case "bitbucket":
		if repo.Auth.Username != "" && repo.Auth.Password != "" {
			return &http.BasicAuth{
				Username: repo.Auth.Username,
				Password: repo.Auth.Password,
			}, nil
		} else if repo.Auth.Token != "" {
			return &http.BasicAuth{
				Username: "x-token-auth",
				Password: repo.Auth.Token,
			}, nil
		}
		// This is for public repositories
		return nil, nil
	default:
		// For generic Git repositories, try to use basic auth if provided
		if repo.Auth.Username != "" && (repo.Auth.Password != "" || repo.Auth.Token != "") {
			password := repo.Auth.Password
			if password == "" {
				password = repo.Auth.Token
			}
			return &http.BasicAuth{
				Username: repo.Auth.Username,
				Password: password,
			}, nil
		}
		// For public repositories or SSH (which is handled by go-git)
		return nil, nil
	}
}
