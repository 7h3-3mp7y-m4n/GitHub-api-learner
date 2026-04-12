package main

import (
	"bufio"
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
	Workflows []Workflow `json:"workflows"`
}

type WorkflowsResponse struct {
	WorkflowRuns []Run `json:"workflow_runs"`
}

type FailedJob struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Conclusion string `json:"conclusion"`
	HTMLURL    string `json:"html_url"`
	LogSnippet string `json:"log_snippet"`
}

type Run struct {
	ID         int         `json:"id"`
	Name       string      `json:"name"`
	Status     string      `json:"status"`
	Conclusion string      `json:"conclusion"`
	RunNumber  int         `json:"run_number"`
	CreatedAt  time.Time   `json:"created_at"`
	UpdatedAt  time.Time   `json:"updated_at"`
	HTMLURL    string      `json:"html_url"`
	FailedJobs []FailedJob `json:"failed_jobs,omitempty"`
}

type Job struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	HTMLURL    string `json:"html_url"`
}

type JobsResponse struct {
	Jobs []Job `json:"jobs"`
}

type Config struct {
	Settings struct {
		SourceRepo         string `yaml:"source_repo"`
		MaxRunsPerWorkflow int    `yaml:"max_runs_per_workflow"`
		RecentRunsInOutput int    `yaml:"recent_runs_in_output"`
	} `yaml:"settings"`

	Workflows []struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
		Critical    bool   `yaml:"critical"`
		Required    bool   `yaml:"required"`
	} `yaml:"workflows"`
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
	RecentRuns      []Run    `json:"recent_runs"`
}

type DashboardData struct {
	GeneratedAt   time.Time         `json:"generated_at"`
	Repo          string            `json:"repo"`
	OverallHealth float64           `json:"overall_health"`
	Workflows     []WorkflowSummary `json:"workflows"`
}

type Client struct {
	token   string
	repo    string
	http    *http.Client
	logHTTP *http.Client
}

func NewClient(token, repo string) *Client {
	return &Client{
		token:   token,
		repo:    repo,
		http:    &http.Client{Timeout: 20 * time.Second},
		logHTTP: &http.Client{Timeout: 2 * time.Minute},
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
	return json.NewDecoder(resp.Body).Decode(v)
}

func (c *Client) fetchLogLines(logURL string, maxLines int) ([]string, error) {
	var resp *http.Response
	var err error

	for attempt := 0; attempt < 2; attempt++ {
		req, _ := http.NewRequest("GET", logURL, nil)
		if c.token != "" {
			req.Header.Set("Authorization", "Bearer "+c.token)
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		resp, err = c.logHTTP.Do(req)
		if err == nil {
			break
		}
		if attempt == 0 {
			log.Printf("log fetch attempt 1 failed (%v), retrying...", err)
			time.Sleep(3 * time.Second)
		}
	}
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var matched []string
	var tail []string

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 256*1024), 256*1024)

	for scanner.Scan() {
		clean := stripGHTimestamp(scanner.Text())
		if clean == "" {
			continue
		}

		tail = append(tail, clean)
		if len(tail) > 20 {
			tail = tail[1:]
		}

		if len(matched) < maxLines {
			lower := strings.ToLower(clean)
			if strings.Contains(lower, "error") ||
				strings.Contains(lower, "fatal") ||
				strings.Contains(lower, "failed") ||
				strings.Contains(lower, "panic") ||
				strings.Contains(lower, "exit code") {
				matched = append(matched, clean)
			}
		}
	}

	if serr := scanner.Err(); serr != nil {
		log.Printf("    warn: scanner error reading log: %v (returning partial result)", serr)
	}

	if len(matched) == 0 {
		return tail, nil
	}
	return matched, nil
}

func (c *Client) listWorkflows() ([]Workflow, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/actions/workflows", c.repo)
	var resp WorkflowsListResponse
	err := c.get(url, &resp)
	return resp.Workflows, err
}

func (c *Client) fetchRuns(workflowID int, limit int) ([]Run, error) {
	url := fmt.Sprintf(
		"https://api.github.com/repos/%s/actions/workflows/%d/runs?per_page=%d",
		c.repo, workflowID, limit,
	)
	var resp WorkflowsResponse
	err := c.get(url, &resp)
	return resp.WorkflowRuns, err
}

func (c *Client) enrichWithLogs(run *Run) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/actions/runs/%d/jobs", c.repo, run.ID)
	var resp JobsResponse
	if err := c.get(url, &resp); err != nil {
		log.Printf("  warn: could not fetch jobs for run %d: %v", run.ID, err)
		return
	}

	for _, job := range resp.Jobs {
		if job.Conclusion != "failure" {
			continue
		}
		logURL := fmt.Sprintf("https://api.github.com/repos/%s/actions/jobs/%d/logs", c.repo, job.ID)
		log.Printf("fetching logs for failed job %d (%s)...", job.ID, job.Name)
		lines, err := c.fetchLogLines(logURL, 30)
		snippet := ""
		if err != nil {
			log.Printf("warn: could not fetch logs for job %d: %v", job.ID, err)
		} else {
			snippet = strings.Join(lines, "\n")
		}
		run.FailedJobs = append(run.FailedJobs, FailedJob{
			ID:         job.ID,
			Name:       job.Name,
			Conclusion: job.Conclusion,
			HTMLURL:    job.HTMLURL,
			LogSnippet: snippet,
		})
	}
}

func stripGHTimestamp(line string) string {
	if len(line) > 29 && line[10] == 'T' {
		line = line[29:]
	}
	return strings.TrimSpace(line)
}

func normalize(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, "_", "")
	s = strings.ReplaceAll(s, " ", "")
	return s
}

func stemPath(path string) string {
	parts := strings.Split(path, "/")
	file := parts[len(parts)-1]
	file = strings.TrimSuffix(file, ".yml")
	file = strings.TrimSuffix(file, ".yaml")
	return normalize(file)
}

func findWorkflow(workflows []Workflow, keyword string) *Workflow {
	key := normalize(keyword)
	for i, wf := range workflows {
		if stemPath(wf.Path) == key {
			return &workflows[i]
		}
	}
	for i, wf := range workflows {
		stem := stemPath(wf.Path)
		if strings.Contains(stem, key) || strings.Contains(key, stem) {
			return &workflows[i]
		}
	}
	for i, wf := range workflows {
		name := normalize(wf.Name)
		if strings.Contains(name, key) || strings.Contains(key, name) {
			return &workflows[i]
		}
	}
	return nil
}

func buildWeatherHistory(runs []Run) []string {
	const slots = 7
	history := make([]string, slots)
	for i := range history {
		history[i] = "unknown"
	}
	take := runs
	if len(take) > slots {
		take = runs[:slots]
	}
	for i, r := range take {
		idx := len(take) - 1 - i
		c := r.Conclusion
		if c == "" {
			c = "unknown"
		}
		switch c {
		case "success", "failure", "skipped", "action_required":
		default:
			c = "unknown"
		}
		history[slots-len(take)+idx] = c
	}
	return history
}

func buildSummary(runs []Run, name, desc string, critical, required bool) WorkflowSummary {
	var failed int
	var totalDuration float64
	for _, r := range runs {
		if r.Conclusion == "failure" {
			failed++
		}
		totalDuration += r.UpdatedAt.Sub(r.CreatedAt).Seconds()
	}
	total := len(runs)
	var failureRate, avg float64
	if total > 0 {
		failureRate = float64(failed) / float64(total) * 100
		avg = totalDuration / float64(total)
	}
	var lastRun *Run
	if len(runs) > 0 {
		r := runs[0]
		lastRun = &r
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
		WeatherHistory:  buildWeatherHistory(runs),
		LastRun:         lastRun,
		RecentRuns:      runs,
	}
}

func main() {
	cfgBytes, _ := os.ReadFile("config.yaml")
	var cfg Config
	yaml.Unmarshal(cfgBytes, &cfg)

	recentLimit := cfg.Settings.RecentRunsInOutput
	if recentLimit <= 0 {
		recentLimit = 7
	}

	client := NewClient(os.Getenv("GITHUB_TOKEN"), cfg.Settings.SourceRepo)

	workflows, err := client.listWorkflows()
	if err != nil {
		log.Fatal(err)
	}

	var summaries []WorkflowSummary
	var totalHealth float64

	for _, w := range cfg.Workflows {
		wf := findWorkflow(workflows, w.Name)
		if wf == nil {
			log.Println("Not found:", w.Name)
			continue
		}
		log.Printf("Matched: %s -> %s", w.Name, wf.Name)

		runs, _ := client.fetchRuns(wf.ID, cfg.Settings.MaxRunsPerWorkflow)
		sort.Slice(runs, func(i, j int) bool {
			return runs[i].CreatedAt.After(runs[j].CreatedAt)
		})

		recent := runs
		if len(recent) > recentLimit {
			recent = runs[:recentLimit]
		}

		log.Printf("fetching logs for failed runs in %s…", w.Name)
		for i := range recent {
			if recent[i].Conclusion == "failure" {
				client.enrichWithLogs(&recent[i])
			}
		}

		summary := buildSummary(runs, w.Name, w.Description, w.Critical, w.Required)
		summary.RecentRuns = recent
		summaries = append(summaries, summary)
		totalHealth += (100 - summary.FailureRate)
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
	}

	out, _ := json.MarshalIndent(data, "", "  ")
	os.WriteFile("stats.json", out, 0644)
	log.Println("stats.json generated ...")
	log.Fatal(http.ListenAndServe(":8080", http.FileServer(http.Dir("."))))
}
