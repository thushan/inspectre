package analyser

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LLMAnalyser analyses a repository using an LLM API
type LLMAnalyser struct {
	name      string
	apiKey    string
	apiURL    string
	modelName string
	maxTokens int
	fileLimit int
	sizeLimit int64 // in bytes
}

const (
	DefaultMaxTokens = 4096
	DefaultFileLimit = 10
	DefaultSizeLimit = 100 * 1024 // 100KB
)

// NewLLMAnalyser creates a new LLM analyser
func NewLLMAnalyser() *LLMAnalyser {
	apiKey := os.Getenv("INSPECTRE_LLM_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}

	maxTokens := DefaultMaxTokens
	fileLimit := DefaultFileLimit
	sizeLimit := DefaultSizeLimit

	if tokenStr := os.Getenv("INSPECTRE_LLM_MAX_TOKENS"); tokenStr != "" {
		if parsed, err := fmt.Sscanf(tokenStr, "%d", &maxTokens); err != nil || parsed != 1 {
			maxTokens = DefaultMaxTokens
		}
	}

	if limitStr := os.Getenv("INSPECTRE_LLM_FILE_LIMIT"); limitStr != "" {
		if parsed, err := fmt.Sscanf(limitStr, "%d", &fileLimit); err != nil || parsed != 1 {
			fileLimit = DefaultFileLimit
		}
	}

	if sizeStr := os.Getenv("INSPECTRE_LLM_SIZE_LIMIT"); sizeStr != "" {
		var sizeLimitKB int
		if parsed, err := fmt.Sscanf(sizeStr, "%d", &sizeLimitKB); err == nil && parsed == 1 {
			sizeLimit = sizeLimitKB * 1024
		}
	}

	// TODO: Refactor and support openrouter
	return &LLMAnalyser{
		name:      "llm_analyser",
		apiKey:    apiKey,
		apiURL:    "https://api.openai.com/v1/chat/completions",
		modelName: "gpt-4",
		maxTokens: maxTokens,
		fileLimit: fileLimit,
		sizeLimit: int64(sizeLimit),
	}
}

// Name returns the analyser identifier
func (a *LLMAnalyser) Name() string {
	return a.name
}

// Initialize prepares the analyser
func (a *LLMAnalyser) Initialize(repoPath string, env map[string]string) error {
	if a.apiKey == "" {
		return fmt.Errorf("LLM API key not set, please set INSPECTRE_LLM_API_KEY environment variable")
	}

	if modelName, ok := env["INSPECTRE_LLM_MODEL"]; ok && modelName != "" {
		a.modelName = modelName
	}

	if apiURL, ok := env["INSPECTRE_LLM_API_URL"]; ok && apiURL != "" {
		a.apiURL = apiURL
	}

	return nil
}

// Run performs the analyser
func (a *LLMAnalyser) Run() ([]Metric, error) {
	// TODO: This would be implemented to call the LLM API
	// For now, return a placeholder metric
	return []Metric{
		{
			Name:      "llm_analysis_status",
			Value:     "not_implemented",
			Timestamp: time.Now(),
		},
	}, nil
}

// AnalyseRepository performs a full repository analyser
func (a *LLMAnalyser) AnalyseRepository(repoPath string) (*RepoInsights, error) {
	if a.apiKey == "" {
		return nil, fmt.Errorf("LLM API key not set, please set INSPECTRE_LLM_API_KEY environment variable")
	}

	files, err := a.selectImportantFiles(repoPath)
	if err != nil {
		return nil, fmt.Errorf("file selection failed: %w", err)
	}

	fileContents := make(map[string]string)
	for _, path := range files {
		relPath, err := filepath.Rel(repoPath, path)
		if err != nil {
			relPath = path
		}

		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		fileContents[relPath] = string(data)
	}

	prompt := a.buildAnalysisPrompt(repoPath, fileContents)

	insights, err := a.callLLMAPI(prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM API call failed: %w", err)
	}

	return insights, nil
}

// selectImportantFiles selects the most relevant files for analyser
func (a *LLMAnalyser) selectImportantFiles(repoPath string) ([]string, error) {
	importantPatterns := []string{
		"README.md", "README", "readme.md",
		"go.mod", "go.sum", "package.json", "requirements.txt",
		"Dockerfile", "docker-compose.yml", "Makefile",
		".gitignore", ".golangci.yml",
	}

	// Find important files first
	var importantFiles []string
	for _, pattern := range importantPatterns {
		matches, _ := filepath.Glob(filepath.Join(repoPath, pattern))
		for _, match := range matches {
			info, err := os.Stat(match)
			if err != nil || info.IsDir() || info.Size() > a.sizeLimit {
				continue
			}
			importantFiles = append(importantFiles, match)
			if len(importantFiles) >= a.fileLimit {
				return importantFiles, nil
			}
		}
	}

	// Look for other potentially important files
	err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files with errors
		}

		// Skip directories and excluded paths
		if info.IsDir() {
			// TODO: Make this configurable
			if strings.HasPrefix(filepath.Base(path), ".") ||
				filepath.Base(path) == "node_modules" ||
				filepath.Base(path) == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}

		if info.Size() > a.sizeLimit {
			return nil
		}

		for _, f := range importantFiles {
			if f == path {
				return nil
			}
		}

		ext := strings.ToLower(filepath.Ext(path))
		// TODO: Make this configurable
		if ext == ".go" || ext == ".js" || ext == ".py" || ext == ".java" || ext == ".ts" {
			importantFiles = append(importantFiles, path)
		}

		if len(importantFiles) >= a.fileLimit {
			return io.EOF
		}

		return nil
	})

	if err != nil && err != io.EOF {
		return nil, err
	}

	return importantFiles, nil
}

// buildAnalysisPrompt creates a prompt for the LLM
func (a *LLMAnalyser) buildAnalysisPrompt(repoPath string, fileContents map[string]string) string {
	var sb strings.Builder

	sb.WriteString("Analyse the following repository files and provide insights about the codebase. ")
	sb.WriteString("Focus on architecture, code quality, potential issues, and recommendations.\n\n")
	sb.WriteString("Format your response as JSON with the following structure:\n")
	sb.WriteString("{\n")
	sb.WriteString("  \"summary\": \"Brief overview of the repository\",\n")
	sb.WriteString("  \"key_findings\": [\"Finding 1\", \"Finding 2\", ...],\n")
	sb.WriteString("  \"recommendations\": [\"Recommendation 1\", \"Recommendation 2\", ...],\n")
	sb.WriteString("  \"architecture_score\": 0-100,\n")
	sb.WriteString("  \"primary_language\": \"Main programming language\",\n")
	sb.WriteString("  \"languages\": {\"language1\": percentage, ...},\n")
	sb.WriteString("  \"frameworks\": [\"Framework 1\", ...],\n")
	sb.WriteString("  \"security_concerns\": [\"Concern 1\", ...]\n")
	sb.WriteString("}\n\n")

	sb.WriteString("Repository files:\n\n")

	for path, content := range fileContents {
		sb.WriteString(fmt.Sprintf("=== BEGIN FILE: %s ===\n", path))
		sb.WriteString(content)
		sb.WriteString(fmt.Sprintf("\n=== END FILE: %s ===\n\n", path))
	}

	return sb.String()
}

// callLLMAPI calls the LLM API with the given prompt
func (a *LLMAnalyser) callLLMAPI(prompt string) (*RepoInsights, error) {
	reqBody := map[string]interface{}{
		"model": a.modelName,
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "You are a code analyser assistant that provides insights about repositories.",
			},
			{
				"role":    "user",
				"content": prompt,
			},
		},
		// TODO: Make these configurable
		"max_tokens":        a.maxTokens,
		"temperature":       0.2,
		"top_p":             1,
		"frequency_penalty": 0,
		"presence_penalty":  0,
	}

	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", a.apiURL, bytes.NewBuffer(reqJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", a.apiKey))

	// TODO: Make this configurable
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned error status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse API response: %w", err)
	}

	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("API returned empty choices")
	}

	content := apiResp.Choices[0].Message.Content

	insights, err := a.parseInsightsFromLLMResponse(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse insights: %w", err)
	}

	return insights, nil
}

// parseInsightsFromLLMResponse extracts JSON from LLM response
func (a *LLMAnalyser) parseInsightsFromLLMResponse(content string) (*RepoInsights, error) {
	jsonStart := strings.Index(content, "{")
	jsonEnd := strings.LastIndex(content, "}")

	if jsonStart == -1 || jsonEnd == -1 || jsonEnd <= jsonStart {
		return nil, fmt.Errorf("could not find valid JSON in response")
	}

	jsonStr := content[jsonStart : jsonEnd+1]

	var insights RepoInsights
	if err := json.Unmarshal([]byte(jsonStr), &insights); err != nil {
		return nil, fmt.Errorf("failed to parse insights JSON: %w", err)
	}

	insights.GeneratedTimestamp = time.Now()

	return &insights, nil
}

// Cleanup performs any necessary cleanup
func (a *LLMAnalyser) Cleanup() error {
	return nil
}

// RepoInsights contains the analyser results
type RepoInsights struct {
	Summary            string                 `json:"summary"`
	KeyFindings        []string               `json:"key_findings"`
	Recommendations    []string               `json:"recommendations"`
	ArchitectureScore  int                    `json:"architecture_score"`
	PrimaryLanguage    string                 `json:"primary_language"`
	Languages          map[string]float64     `json:"languages"`
	Frameworks         []string               `json:"frameworks"`
	SecurityConcerns   []string               `json:"security_concerns,omitempty"`
	TechnicalDebt      map[string]interface{} `json:"technical_debt,omitempty"`
	GeneratedTimestamp time.Time              `json:"generated_timestamp"`
}
