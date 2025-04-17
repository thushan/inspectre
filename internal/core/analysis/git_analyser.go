package analysis

import (
	"fmt"
	"sort"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// GitAnalyser examines git repository metadata
type GitAnalyser struct {
	repoPath       string
	repo           *git.Repository
	commitCount    int
	branchCount    int
	tagCount       int
	contributors   map[string]int
	latestCommit   time.Time
	earliestCommit time.Time
}

// NewGitAnalyser creates a new git metadata analyser
func NewGitAnalyser() *GitAnalyser {
	return &GitAnalyser{
		contributors: make(map[string]int),
	}
}

// Name returns the analyser identifier
func (a *GitAnalyser) Name() string {
	return "git_analyser"
}

// Initialize prepares the analyser
func (a *GitAnalyser) Initialize(repoPath string, env map[string]string) error {
	a.repoPath = repoPath

	// Open the repository
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return fmt.Errorf("failed to open git repository: %w", err)
	}
	a.repo = repo

	return nil
}

// Run performs the analysis
func (a *GitAnalyser) Run() ([]Metric, error) {
	var metrics []Metric
	now := time.Now()

	// Analyse commits
	if err := a.analyseCommits(); err != nil {
		return nil, err
	}

	// Count branches
	if err := a.countBranches(); err != nil {
		return nil, err
	}

	// Count tags
	if err := a.countTags(); err != nil {
		return nil, err
	}

	// Add basic metrics
	metrics = append(metrics, Metric{
		Name:      "commit_count",
		Value:     a.commitCount,
		Timestamp: now,
	})

	metrics = append(metrics, Metric{
		Name:      "branch_count",
		Value:     a.branchCount,
		Timestamp: now,
	})

	metrics = append(metrics, Metric{
		Name:      "tag_count",
		Value:     a.tagCount,
		Timestamp: now,
	})

	metrics = append(metrics, Metric{
		Name:      "repository_age_days",
		Value:     int(now.Sub(a.earliestCommit).Hours() / 24),
		Timestamp: now,
	})

	// Add top contributors
	topContributors := a.getTopContributors(5)
	for _, c := range topContributors {
		metrics = append(metrics, Metric{
			Name:      "contributor_commits",
			Key:       c.Email,
			Value:     c.Commits,
			Labels:    map[string]string{"email": c.Email},
			Timestamp: now,
		})
	}

	return metrics, nil
}

// analyseCommits processes the commit history
func (a *GitAnalyser) analyseCommits() error {
	// Get the HEAD reference
	ref, err := a.repo.Head()
	if err != nil {
		return fmt.Errorf("failed to get HEAD: %w", err)
	}

	// Get commit history
	commits, err := a.repo.Log(&git.LogOptions{From: ref.Hash()})
	if err != nil {
		return fmt.Errorf("failed to get commit history: %w", err)
	}

	// Process each commit
	a.earliestCommit = time.Now() // Start with current time
	err = commits.ForEach(func(c *object.Commit) error {
		a.commitCount++

		// Track contributor stats
		email := c.Author.Email
		a.contributors[email]++

		// Track first and last commit dates
		if a.latestCommit.IsZero() || c.Author.When.After(a.latestCommit) {
			a.latestCommit = c.Author.When
		}

		if c.Author.When.Before(a.earliestCommit) {
			a.earliestCommit = c.Author.When
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to analyse commits: %w", err)
	}

	return nil
}

// countBranches counts the number of branches
func (a *GitAnalyser) countBranches() error {
	branches, err := a.repo.Branches()
	if err != nil {
		return fmt.Errorf("failed to get branches: %w", err)
	}

	// Count branches
	err = branches.ForEach(func(ref *plumbing.Reference) error {
		a.branchCount++
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to count branches: %w", err)
	}

	return nil
}

// countTags counts the number of tags
func (a *GitAnalyser) countTags() error {
	tags, err := a.repo.Tags()
	if err != nil {
		return fmt.Errorf("failed to get tags: %w", err)
	}

	// Count tags
	err = tags.ForEach(func(ref *plumbing.Reference) error {
		a.tagCount++
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to count tags: %w", err)
	}

	return nil
}

// Contributor represents a repository contributor
type Contributor struct {
	Email   string
	Commits int
}

// getTopContributors returns the top N contributors by commit count
func (a *GitAnalyser) getTopContributors(n int) []Contributor {
	var contributors []Contributor

	for email, commits := range a.contributors {
		contributors = append(contributors, Contributor{
			Email:   email,
			Commits: commits,
		})
	}

	// Sort by commit count (descending)
	sort.Slice(contributors, func(i, j int) bool {
		return contributors[i].Commits > contributors[j].Commits
	})

	// Return top N (or all if less than N)
	if len(contributors) > n {
		contributors = contributors[:n]
	}

	return contributors
}

// Cleanup performs any necessary cleanup
func (a *GitAnalyser) Cleanup() error {
	// No cleanup needed for this analyser
	return nil
}
