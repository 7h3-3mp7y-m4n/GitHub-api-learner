package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

type NotifyConfig struct {
	Enabled             bool   `yaml:"enabled"`
	TargetRepo          string `yaml:"target_repo"`
	Label               string `yaml:"label"`
	ConsecutiveFailures int    `yaml:"consecutive_failures"`
}

type Issue struct {
	Number  int    `json:"number"`
	Title   string `json:"title"`
	State   string `json:"state"`
	HTMLURL string `json:"html_url"`
}

type IssuesResponse []Issue

type Notifier struct {
	token      string
	sourceRepo string
	targetRepo string
	label      string
	threshold  int
	http       *http.Client
}

func NewNotifier(token, sourceRepo string, cfg NotifyConfig) *Notifier {
	targetRepo := cfg.TargetRepo
	if targetRepo == "" {
		targetRepo = sourceRepo
	}
	label := cfg.Label
	if label == "" {
		label = "ci-failure"
	}
	threshold := cfg.ConsecutiveFailures
	if threshold <= 0 {
		threshold = 1
	}
	return &Notifier{
		token:      token,
		sourceRepo: sourceRepo,
		targetRepo: targetRepo,
		label:      label,
		threshold:  threshold,
		http:       &http.Client{Timeout: 15 * time.Second},
	}
}

func consecutiveFailures(history []string) int {
	count := 0
	for i := len(history) - 1; i >= 0; i-- {
		if history[i] == "failure" {
			count++
		} else {
			break
		}
	}
	return count
}

func (n *Notifier) Process(summary WorkflowSummary) {
	log.Printf("[notify] ── %q ───────────────────────────", summary.Name)
	log.Printf("[notify]   critical=%v  last_run=%v",
		summary.Critical, summary.LastRun != nil)

	if !summary.Critical {
		log.Printf("[notify]   skip: not critical")
		return
	}
	if summary.LastRun == nil {
		log.Printf("[notify]   skip: no runs yet")
		return
	}

	log.Printf("[notify]   last_run=#%d conclusion=%q",
		summary.LastRun.RunNumber, summary.LastRun.Conclusion)
	log.Printf("[notify]   weather_history=%v", summary.WeatherHistory)

	consecutive := consecutiveFailures(summary.WeatherHistory)
	log.Printf("[notify]   consecutive=%d  threshold=%d", consecutive, n.threshold)

	if summary.LastRun.Conclusion != "failure" {
		log.Printf("[notify]   skip: last run passed — close issue manually if one is open")
		return
	}

	log.Printf("[notify]   searching for open issue %q in %s...",
		issueTitle(summary.Name), n.targetRepo)
	existing := n.findOpenIssue(summary.Name)

	switch {
	case existing == nil && consecutive >= n.threshold:
		log.Printf("[notify]  -> creating issue (consecutive=%d >= threshold=%d)",
			consecutive, n.threshold)
		n.createIssue(summary, consecutive)

	case existing != nil:
		log.Printf("[notify]  -> commenting on existing issue #%d", existing.Number)
		n.addComment(existing.Number, summary, consecutive)

	default:
		log.Printf("[notify]   -> no action: threshold not reached (%d/%d)",
			consecutive, n.threshold)
	}
}

func (n *Notifier) apiURL(path string) string {
	return "https://api.github.com/repos/" + n.targetRepo + path
}

func (n *Notifier) do(method, url string, body interface{}) (*http.Response, error) {
	var buf *bytes.Buffer
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		buf = bytes.NewBuffer(b)
	} else {
		buf = bytes.NewBuffer(nil)
	}

	req, err := http.NewRequest(method, url, buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+n.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	return n.http.Do(req)
}

func (n *Notifier) findOpenIssue(workflowName string) *Issue {
	url := n.apiURL(fmt.Sprintf("/issues?state=open&labels=%s&per_page=50", n.label))
	log.Printf("[notify]   GET %s", url)

	resp, err := n.do("GET", url, nil)
	if err != nil {
		log.Printf("[notify]   ERROR listing issues: %v", err)
		return nil
	}
	defer resp.Body.Close()

	log.Printf("[notify]   issues list status: %s", resp.Status)
	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 401 {
		log.Printf("[notify]   ERROR 401 Unauthorized — GITHUB_TOKEN is invalid or missing scopes")
		log.Printf("[notify]   response: %s", string(raw))
		return nil
	}
	if resp.StatusCode == 404 {
		log.Printf("[notify]   ERROR 404 — target_repo %q not found or token has no access to it", n.targetRepo)
		log.Printf("[notify]   response: %s", string(raw))
		return nil
	}
	if resp.StatusCode >= 300 {
		log.Printf("[notify]   ERROR %s — response: %s", resp.Status, string(raw))
		return nil
	}

	var issues IssuesResponse
	if err := json.Unmarshal(raw, &issues); err != nil {
		log.Printf("[notify]   ERROR decoding issues: %v", err)
		log.Printf("[notify]   raw body: %s", string(raw))
		return nil
	}

	log.Printf("[notify]   %d open issue(s) with label %q", len(issues), n.label)

	needle := issueTitle(workflowName)
	for _, issue := range issues {
		log.Printf("[notify]   checking #%d %q", issue.Number, issue.Title)
		if issue.Title == needle {
			return &issue
		}
	}
	return nil
}

func (n *Notifier) ensureLabel() {
	resp, err := n.do("POST", n.apiURL("/labels"), map[string]string{
		"name":        n.label,
		"color":       "d73a49",
		"description": "Automated CI failure notification",
	})
	if err != nil {
		log.Printf("[notify]   ensureLabel error: %v", err)
		return
	}
	defer resp.Body.Close()
	log.Printf("[notify]   ensureLabel: %s", resp.Status)
}

func (n *Notifier) createIssue(summary WorkflowSummary, consecutive int) {
	n.ensureLabel()

	title := issueTitle(summary.Name)
	body := buildIssueBody(summary, consecutive, n.sourceRepo)

	log.Printf("[notify]   POST /issues title=%q", title)

	resp, err := n.do("POST", n.apiURL("/issues"), map[string]interface{}{
		"title":  title,
		"body":   body,
		"labels": []string{n.label},
	})
	if err != nil {
		log.Printf("[notify]   ERROR creating issue: %v", err)
		return
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	log.Printf("[notify]   createIssue status: %s", resp.Status)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("[notify]   ERROR body: %s", string(raw))
		return
	}

	var created Issue
	if err := json.Unmarshal(raw, &created); err != nil {
		log.Printf("[notify]   ERROR decoding response: %v — body: %s", err, string(raw))
		return
	}

	log.Printf("[notify]   ✓ issue #%d created → %s", created.Number, created.HTMLURL)
}

func (n *Notifier) addComment(issueNumber int, summary WorkflowSummary, consecutive int) {
	body := buildCommentBody(summary, consecutive, n.sourceRepo)
	url := n.apiURL(fmt.Sprintf("/issues/%d/comments", issueNumber))

	log.Printf("[notify]   POST %s", url)

	resp, err := n.do("POST", url, map[string]string{"body": body})
	if err != nil {
		log.Printf("[notify]   ERROR adding comment: %v", err)
		return
	}
	defer resp.Body.Close()

	log.Printf("[notify]   addComment status: %s", resp.Status)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		log.Printf("[notify]   ERROR body: %s", string(raw))
	}
}

// ── Message builders ──────────────────────────────────────────────────────────

func issueTitle(workflowName string) string {
	return fmt.Sprintf("CI Failure: %s", workflowName)
}

func buildIssueBody(summary WorkflowSummary, consecutive int, sourceRepo string) string {
	lr := summary.LastRun
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("##Critical workflow failing: `%s`\n\n", summary.Name))
	sb.WriteString(fmt.Sprintf("**%d consecutive failure(s)** detected by the CI dashboard.\n\n", consecutive))

	sb.WriteString("### Latest Run\n\n")
	sb.WriteString("| Field | Value |\n|---|---|\n")
	sb.WriteString(fmt.Sprintf("| Run | [#%d](%s) |\n", lr.RunNumber, lr.HTMLURL))
	sb.WriteString(fmt.Sprintf("| Started | %s |\n", lr.CreatedAt.Format(time.RFC1123)))
	sb.WriteString(fmt.Sprintf("| Conclusion | `%s` |\n", lr.Conclusion))
	sb.WriteString(fmt.Sprintf("| Repo | [%s](https://github.com/%s) |\n\n", sourceRepo, sourceRepo))

	if len(lr.FailedJobs) > 0 {
		sb.WriteString("### Failed Jobs\n\n")
		for _, job := range lr.FailedJobs {
			sb.WriteString(fmt.Sprintf("#### ✗ [%s](%s)\n\n", job.Name, job.HTMLURL))
			if job.LogSnippet != "" {
				sb.WriteString("```\n")
				sb.WriteString(job.LogSnippet)
				sb.WriteString("\n```\n\n")
			}
		}
	}

	sb.WriteString("---\n")
	sb.WriteString("This issue was opened automatically by the CI dashboard. ")
	sb.WriteString("Please close it manually once the issue is resolved._\n")

	return sb.String()
}

func buildCommentBody(summary WorkflowSummary, consecutive int, sourceRepo string) string {
	lr := summary.LastRun
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("###Still failing — run [#%d](%s)\n\n", lr.RunNumber, lr.HTMLURL))
	sb.WriteString(fmt.Sprintf("**%d consecutive failure(s)** as of %s.\n\n",
		consecutive, lr.CreatedAt.Format(time.RFC1123)))

	if len(lr.FailedJobs) > 0 {
		for _, job := range lr.FailedJobs {
			sb.WriteString(fmt.Sprintf("**✗ [%s](%s)**\n\n", job.Name, job.HTMLURL))
			if job.LogSnippet != "" {
				sb.WriteString("```\n")
				sb.WriteString(job.LogSnippet)
				sb.WriteString("\n```\n\n")
			}
		}
	}

	return sb.String()
}
