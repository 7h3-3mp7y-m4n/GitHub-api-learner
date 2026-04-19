package main

import (
	"bytes"
	"encoding/json"
	"fmt"
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
	sourceRepo string // repo we're monitoring (for run links)
	targetRepo string // repo where we open issues
	label      string
	threshold  int // consecutive failures needed before opening an issue
	http       *http.Client
}

func NewNotifier(token, sourceRepo string, cfg NotifyConfig) *Notifier {
	targetRepo := cfg.TargetRepo
	if targetRepo == "" {
		targetRepo = sourceRepo // default: open issues in the same repo
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
	if !summary.Critical {
		return
	}
	if summary.LastRun == nil {
		return
	}

	isFailing := summary.LastRun.Conclusion == "failure"
	if !isFailing {
		log.Printf("  notify: %q passing — no action (close issue manually if open)", summary.Name)
		return
	}

	consecutive := consecutiveFailures(summary.WeatherHistory)

	existingIssue := n.findOpenIssue(summary.Name)

	if existingIssue == nil && consecutive >= n.threshold {
		log.Printf("notify: opening issue for %q (%d consecutive failures)", summary.Name, consecutive)
		n.createIssue(summary, consecutive)
	} else if existingIssue != nil {
		log.Printf("notify: commenting on issue #%d for %q", existingIssue.Number, summary.Name)
		n.addComment(existingIssue.Number, summary, consecutive)
	} else {
		log.Printf("notify: %q failing but threshold not yet reached (%d/%d)", summary.Name, consecutive, n.threshold)
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
	resp, err := n.do("GET", url, nil)
	if err != nil {
		log.Printf("  notify: warn: could not list issues: %v", err)
		return nil
	}
	defer resp.Body.Close()

	var issues IssuesResponse
	if err := json.NewDecoder(resp.Body).Decode(&issues); err != nil {
		log.Printf("  notify: warn: could not decode issues: %v", err)
		return nil
	}

	needle := issueTitle(workflowName)
	for _, issue := range issues {
		if issue.Title == needle {
			return &issue
		}
	}
	return nil
}

func (n *Notifier) ensureLabel() {
	url := n.apiURL("/labels")
	resp, err := n.do("POST", url, map[string]string{
		"name":        n.label,
		"color":       "d73a49",
		"description": "Automated CI failure notification",
	})
	if err != nil || resp == nil {
		return
	}
	resp.Body.Close()
}

func (n *Notifier) createIssue(summary WorkflowSummary, consecutive int) {
	n.ensureLabel()

	title := issueTitle(summary.Name)
	body := buildIssueBody(summary, consecutive, n.sourceRepo)

	resp, err := n.do("POST", n.apiURL("/issues"), map[string]interface{}{
		"title":  title,
		"body":   body,
		"labels": []string{n.label},
	})
	if err != nil {
		log.Printf("  notify: warn: could not create issue: %v", err)
		return
	}
	defer resp.Body.Close()

	var created Issue
	json.NewDecoder(resp.Body).Decode(&created)
	if created.Number > 0 {
		log.Printf("  notify: issue #%d created → %s", created.Number, created.HTMLURL)
	}
}

func (n *Notifier) addComment(issueNumber int, summary WorkflowSummary, consecutive int) {
	body := buildCommentBody(summary, consecutive, n.sourceRepo)
	url := n.apiURL(fmt.Sprintf("/issues/%d/comments", issueNumber))

	resp, err := n.do("POST", url, map[string]string{"body": body})
	if err != nil {
		log.Printf("  notify: warn: could not add comment: %v", err)
		return
	}
	resp.Body.Close()
}

func issueTitle(workflowName string) string {
	return fmt.Sprintf("CI Failure: %s", workflowName)
}

func buildIssueBody(summary WorkflowSummary, consecutive int, sourceRepo string) string {
	lr := summary.LastRun
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("##Critical workflow failing: `%s`\n\n", summary.Name))
	sb.WriteString(fmt.Sprintf("**%d consecutive failure(s)** detected by the CI dashboard.\n\n", consecutive))

	sb.WriteString("### Latest Run\n\n")
	sb.WriteString(fmt.Sprintf("| Field | Value |\n|---|---|\n"))
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
