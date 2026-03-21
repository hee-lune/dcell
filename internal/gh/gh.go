// Package gh provides GitHub CLI integration for dcell.
package gh

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// Client provides GitHub CLI operations.
type Client struct {
	Repo string // owner/repo
}

// New creates a new GitHub CLI client.
func New() (*Client, error) {
	if !HasGH() {
		return nil, fmt.Errorf("gh CLI is not installed")
	}

	repo, err := getCurrentRepo()
	if err != nil {
		return nil, err
	}

	return &Client{Repo: repo}, nil
}

// HasGH checks if gh CLI is available.
func HasGH() bool {
	_, err := exec.LookPath("gh")
	return err == nil
}

// PR represents a GitHub pull request.
type PR struct {
	Number    int    `json:"number"`
	Title     string `json:"title"`
	State     string `json:"state"`
	HeadRef   string `json:"headRefName"`
	BaseRef   string `json:"baseRefName"`
	URL       string `json:"url"`
	IsDraft   bool   `json:"isDraft"`
	Author    Author `json:"author"`
	CreatedAt string `json:"createdAt"`
}

// Author represents PR author.
type Author struct {
	Login string `json:"login"`
}

// ListPRs lists pull requests.
func (c *Client) ListPRs(state string) ([]PR, error) {
	args := []string{"pr", "list", "--json", "number,title,state,headRefName,baseRefName,url,isDraft,author,createdAt"}
	if state != "" {
		args = append(args, "--state", state)
	}

	cmd := exec.Command("gh", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list PRs: %w", err)
	}

	var prs []PR
	if err := json.Unmarshal(out, &prs); err != nil {
		return nil, fmt.Errorf("failed to parse PRs: %w", err)
	}

	return prs, nil
}

// GetPR gets a specific PR by number.
func (c *Client) GetPR(number int) (*PR, error) {
	cmd := exec.Command("gh", "pr", "view", fmt.Sprintf("%d", number), "--json", "number,title,state,headRefName,baseRefName,url,isDraft,author,createdAt")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get PR %d: %w", number, err)
	}

	var pr PR
	if err := json.Unmarshal(out, &pr); err != nil {
		return nil, fmt.Errorf("failed to parse PR: %w", err)
	}

	return &pr, nil
}

// GetPRForBranch gets PR for a specific branch.
func (c *Client) GetPRForBranch(branch string) (*PR, error) {
	cmd := exec.Command("gh", "pr", "view", branch, "--json", "number,title,state,headRefName,baseRefName,url,isDraft,author,createdAt")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("no PR found for branch %s", branch)
	}

	var pr PR
	if err := json.Unmarshal(out, &pr); err != nil {
		return nil, fmt.Errorf("failed to parse PR: %w", err)
	}

	return &pr, nil
}

// ViewPR opens PR in browser.
func (c *Client) ViewPR(number int) error {
	args := []string{"pr", "view"}
	if number > 0 {
		args = append(args, fmt.Sprintf("%d", number))
	}
	args = append(args, "--web")

	cmd := exec.Command("gh", args...)
	return cmd.Run()
}

// CreatePR creates a new pull request.
func (c *Client) CreatePR(title, body string, draft bool) error {
	args := []string{"pr", "create", "--title", title, "--body", body}
	if draft {
		args = append(args, "--draft")
	}

	cmd := exec.Command("gh", args...)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Start()
}

func getCurrentRepo() (string, error) {
	cmd := exec.Command("gh", "repo", "view", "--json", "nameWithOwner", "-q", ".nameWithOwner")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current repo: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
