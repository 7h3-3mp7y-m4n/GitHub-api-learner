package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
)

type Workflow struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}

type WorkflowsResponse struct {
	TotalCount int        `json:"total_count"`
	Workflows  []Workflow `json:"workflows"`
}

func main() {
	token := os.Getenv("Git_Token")

	req, err := http.NewRequest("GET", "https://api.github.com/repos/urunc-dev/urunc/actions/workflows", nil)
	if err != nil {
		log.Fatalf("Unable to create request: %v", err)
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Can't send the request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Unexpected status: %s", resp.Status)
	}

	var result WorkflowsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Fatalf("JSON decode failed: %v", err)
	}

	fmt.Printf("Total workflows: %d\n\n", result.TotalCount)
	for _, wf := range result.Workflows {
		fmt.Printf("ID: %d | Name: %s | File: %s\n",
			wf.ID, wf.Name, wf.Path)
	}
}
