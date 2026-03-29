package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

type Workflow struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}

type WorkflowsListResponse struct {
	TotalCount int        `json:"total_count"`
	Workflows  []Workflow `json:"workflows"`
}

type WorkflowsResponse struct {
	TotalCount   int   `json:"total_count"`
	WorkflowRuns []Run `json:"workflow_runs"`
}

type Run struct {
	ID         int       `json:"id"`
	Name       string    `json:"name"`
	Status     string    `json:"status"`
	Conclusion string    `json:"conclusion"`
	RunNumber  int       `json:"run_number"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	HTMLURL    string    `json:"html_url"`
}

type Config struct {
	Settings struct {
		SourceRepo         string `yaml:"source_repo"`
		HistoryDays        int    `yaml:"history_days"`
		RefreshInterval    int    `yaml:"refresh_interval"`
		MaxRunsPerWorkflow int    `yaml:"max_runs_per_workflow"`
	} `yaml:"settings"`
	Workflows []struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
		Critical    bool   `yaml:"critical"`
	} `yaml:"workflows"`
	RequiredTests []string `yaml:"required_tests"`
}

type WorkflowSummary struct {
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	Critical        bool     `json:"critical"`
	Required        bool     `json:"required"`
	TotalRuns       int      `json:"total_runs"`
	FailedRuns      int      `json:"failed_runs"`
	FailureRate     float64  `json:"failure_rate"`
	AvgDurationSecs float64  `json:"avg_duration_secs"`
	WeatherHistory  []string `json:"weather_history"`
	LastRun         *Run     `json:"last_run"`
}

type DashboardData struct {
	GeneratedAt   time.Time         `json:"generated_at"`
	Repo          string            `json:"repo"`
	OverallHealth float64           `json:"overall_health"`
	Workflows     []WorkflowSummary `json:"workflows"`
	RequiredTests []string          `json:"required_tests"`
}

type Client struct {
	token string
	repo  string
	http  *http.Client
}

func NewClient(token, repo string) *Client {
	return &Client{
		token: token,
		repo:  repo,
		http:  &http.Client{},
	}
}

func (c *Client) get(url string, v interface{}) error {
	req, _ := http.NewRequest("GET", url, nil)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub API %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

func (c *Client) listWorkflows() ([]Workflow, error) {
	url := fmt.Sprintf(
		"https://api.github.com/repos/%s/actions/workflows?per_page=100",
		c.repo,
	)
	var resp WorkflowsListResponse
	if err := c.get(url, &resp); err != nil {
		return nil, err
	}
	return resp.Workflows, nil
}

func (c *Client) fetchRunsByWorkflowID(workflowID int, days int, limit int) ([]Run, error) {
	cutoff := time.Now().AddDate(0, 0, -days)

	url := fmt.Sprintf(
		"https://api.github.com/repos/%s/actions/workflows/%d/runs?per_page=%d",
		c.repo, workflowID, limit,
	)

	log.Printf("Fetching runs: %s", url)

	var resp WorkflowsResponse
	if err := c.get(url, &resp); err != nil {
		return nil, err
	}

	log.Printf("Total runs from API: %d", len(resp.WorkflowRuns))

	sort.Slice(resp.WorkflowRuns, func(i, j int) bool {
		return resp.WorkflowRuns[i].CreatedAt.After(resp.WorkflowRuns[j].CreatedAt)
	})

	var filtered []Run

	for _, r := range resp.WorkflowRuns {
		if r.Status != "completed" {
			continue
		}

		if r.CreatedAt.After(cutoff) || len(filtered) < 5 {
			filtered = append(filtered, r)
		}

		if len(filtered) >= limit {
			break
		}
	}

	log.Printf("Runs after filter: %d", len(filtered))
	return filtered, nil
}

func findWorkflowID(configName string, workflows []Workflow) (int, string, bool) {
	lower := strings.ToLower(configName)

	for _, wf := range workflows {
		if strings.ToLower(wf.Name) == lower {
			return wf.ID, wf.Name, true
		}
	}

	for _, wf := range workflows {
		filename := wf.Path[strings.LastIndex(wf.Path, "/")+1:]
		filename = strings.TrimSuffix(filename, ".yml")
		filename = strings.TrimSuffix(filename, ".yaml")
		if strings.ToLower(filename) == lower {
			return wf.ID, wf.Name, true
		}
	}

	return 0, "", false
}

func buildWeather(runs []Run, days int) []string {
	history := make([]string, days)
	for i := range history {
		history[i] = "unknown"
	}
	now := time.Now()

	for i := 0; i < days; i++ {
		start := now.AddDate(0, 0, -(days - 1 - i))
		end := start.Add(24 * time.Hour)

		for _, r := range runs {
			if r.CreatedAt.After(start) && r.CreatedAt.Before(end) {
				history[i] = r.Conclusion
				break
			}
		}
	}
	return history
}

func buildSummary(runs []Run, name, desc string, critical, required bool, days int) WorkflowSummary {
	var failed int
	var totalDuration float64
	var lastRun *Run

	for i, r := range runs {
		if r.Conclusion == "failure" {
			failed++
		}
		totalDuration += r.UpdatedAt.Sub(r.CreatedAt).Seconds()
		if i == 0 {
			copy := r
			lastRun = &copy
		}
	}

	total := len(runs)
	var failureRate, avg float64
	if total > 0 {
		failureRate = float64(failed) / float64(total) * 100
		avg = totalDuration / float64(total)
	}

	return WorkflowSummary{
		Name:            name,
		Description:     desc,
		Critical:        critical,
		Required:        required,
		TotalRuns:       total,
		FailedRuns:      failed,
		FailureRate:     failureRate,
		AvgDurationSecs: avg,
		WeatherHistory:  buildWeather(runs, days),
		LastRun:         lastRun,
	}
}

func main() {
	cfgBytes, _ := os.ReadFile("config.yaml")
	var cfg Config
	yaml.Unmarshal(cfgBytes, &cfg)

	client := NewClient(os.Getenv("GITHUB_TOKEN"), cfg.Settings.SourceRepo)

	allWorkflows, err := client.listWorkflows()
	if err != nil {
		log.Fatal(err)
	}

	requiredSet := map[string]bool{}
	for _, r := range cfg.RequiredTests {
		requiredSet[r] = true
	}

	var summaries []WorkflowSummary
	var totalHealth float64

	for _, wf := range cfg.Workflows {
		id, realName, found := findWorkflowID(wf.Name, allWorkflows)
		if !found {
			log.Printf("Not found: %s", wf.Name)
			continue
		}

		log.Printf("%s → %s (%d)", wf.Name, realName, id)

		runs, _ := client.fetchRunsByWorkflowID(
			id,
			cfg.Settings.HistoryDays,
			cfg.Settings.MaxRunsPerWorkflow,
		)

		sum := buildSummary(
			runs,
			realName,
			wf.Description,
			wf.Critical,
			requiredSet[wf.Name],
			cfg.Settings.HistoryDays,
		)

		summaries = append(summaries, sum)
		totalHealth += (100 - sum.FailureRate)
	}

	health := 0.0
	if len(summaries) > 0 {
		health = totalHealth / float64(len(summaries))
	}

	data := DashboardData{
		GeneratedAt:   time.Now().UTC(),
		Repo:          cfg.Settings.SourceRepo,
		OverallHealth: health,
		Workflows:     summaries,
		RequiredTests: cfg.RequiredTests,
	}

	out, _ := json.MarshalIndent(data, "", "  ")
	os.WriteFile("stats.json", out, 0644)

	log.Println("stats.json generated")

	// log.Fatal(http.ListenAndServe(":8080", http.FileServer(http.Dir("."))))
}
