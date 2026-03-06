package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

type Workflow struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
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
	CreatedAt  time.Time `json:"created_at"`
	Event      string    `json:"event"`
	HTMLURL    string    `json:"html_url"`
}

// func main() {
// 	token := os.Getenv("Git_Token")

// 	req, err := http.NewRequestWithContext(context.Background(), "GET", "https://api.github.com/repos/urunc-dev/urunc/actions/workflows", nil)
// 	if err != nil {
// 		log.Fatalf("Unable to create request: %v", err)
// 	}

// 	req.Header.Set("Accept", "application/vnd.github+json")
// 	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

// 	if token != "" {
// 		req.Header.Set("Authorization", "Bearer "+token)
// 	}

// 	client := &http.Client{}
// 	resp, err := client.Do(req)
// 	if err != nil {
// 		log.Fatalf("Can't send the request: %v", err)
// 	}
// 	defer resp.Body.Close()

// 	if resp.StatusCode != http.StatusOK {
// 		log.Fatalf("Unexpected status: %s", resp.Status)
// 	}

// 	var result WorkflowsResponse
// 	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
// 		log.Fatalf("JSON decode failed: %v", err)
// 	}

// 	fmt.Printf("Found %d Worflows --- \n\n", result.TotalCount)
// 	for _, wf := range result.Workflows {
// 		url := fmt.Sprintf("https://api.github.com/repos/urunc-dev/urunc/actions/workflows/%d/runs?per_page=10",
// 			wf.ID,
// 		)
// 		req, err := http.NewRequestWithContext(context.Background(), "GET", url, nil)
// 		if err != nil {
// 			fmt.Println("error ", err)
// 		}
// 		req.Header.Set("Accept", "application/vnd.github+json")
// 		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
// 		resp, err := client.Do(req)
// 		if err != nil {
// 			fmt.Println("request error:", err)
// 		}
// 		var runsResp TotalRun
// 		if err := json.NewDecoder(resp.Body).Decode(&runsResp); err != nil {
// 			fmt.Println("decode error ", err)
// 			resp.Body.Close()
// 		}
// 		resp.Body.Close()
// 		if len(runsResp.WorkflowRun) == 0 {
// 			fmt.Println("no runs found\n")
// 		}
// 		for _, run := range runsResp.WorkflowRun {
// 			fmt.Printf(
// 				"   %-30s  %-12s  %-10s  triggered by: %-15s  %s\n",
// 				run.Name,
// 				run.Status,
// 				run.Conclusion,
// 				run.Event,
// 				run.CreatedAt.Format("2006-01-02 15:04"),
// 			)
// 		}
// 	}
// }

func main() {
	token := os.Getenv("GITHUB_TOKEN")
	url := "https://api.github.com/repos/urunc-dev/urunc/actions/runs?per_page=100"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatalf("Unable to create request: %v", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("can't send the request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Fatalf("unexpected status: %s", resp.Status)
	}
	var runs WorkflowsResponse
	if err := json.NewDecoder(resp.Body).Decode(&runs); err != nil {
		log.Fatalf("json decode failed: %v", err)
	}
	file, err := os.Create("stats.json")
	if err != nil {
		log.Fatalf("cant create the json file %v", err)
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", " ")
	if err := encoder.Encode(runs.WorkflowRuns); err != nil {
		log.Fatalf("failed to encode json %v ---> ", err)
	}
	fmt.Printf("Saved %d runs to runs.json\n", len(runs.WorkflowRuns))
	// fmt.Println("starting server on 8080 :)")
	// log.Fatal(http.ListenAndServe(":8080", http.FileServer(http.Dir("."))))
}
