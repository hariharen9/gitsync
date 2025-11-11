package main

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Branch represents a git branch with metadata
type Branch struct {
	Name        string
	Description string
	Behind      int
	Ahead       int
	LastCommit  string
	Selected    bool
	Status      string // "ok", "behind", "conflict", "updated"
}

// IsGitRepo checks if current directory is a git repository
func IsGitRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	err := cmd.Run()
	return err == nil
}

// GetCurrentBranch returns the current branch name
func GetCurrentBranch() (string, error) {
	cmd := exec.Command("git", "branch", "--show-current")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// GetAllBranches returns all local branches
func GetAllBranches() ([]string, error) {
	cmd := exec.Command("git", "branch", "--format=%(refname:short)")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	branches := strings.Split(strings.TrimSpace(string(output)), "\n")
	return branches, nil
}

// GetRemotes returns all configured remotes
func GetRemotes() ([]string, error) {
	cmd := exec.Command("git", "remote")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	remotes := strings.Split(strings.TrimSpace(string(output)), "\n")
	return remotes, nil
}

// DetectBaseBranch tries to find the main branch by querying upstream remote's HEAD
func DetectBaseBranch() (string, error) {
	// First, try to detect upstream remote
	upstream, err := DetectUpstreamRemote()
	if err == nil {
		// Try to get the HEAD branch from upstream remote
		cmd := exec.Command("git", "remote", "show", upstream)
		output, err := cmd.Output()
		if err == nil {
			// Parse output to find "HEAD branch: <branch-name>"
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "HEAD branch:") {
					parts := strings.SplitN(line, ":", 2)
					if len(parts) == 2 {
						branch := strings.TrimSpace(parts[1])
						// Verify this branch exists locally
						branches, err := GetAllBranches()
						if err == nil {
							for _, b := range branches {
								if b == branch {
									return branch, nil
								}
							}
						}
						// Branch exists on remote but not locally, still return it
						return branch, nil
					}
				}
			}
		}
	}
	
	// Fallback: try common branch names
	candidates := []string{"main", "master", "dev-integration", "develop"}
	
	branches, err := GetAllBranches()
	if err != nil {
		return "", err
	}
	
	for _, candidate := range candidates {
		for _, branch := range branches {
			if branch == candidate {
				return candidate, nil
			}
		}
	}
	
	// If none found, return the first branch
	if len(branches) > 0 {
		return branches[0], nil
	}
	
	return "", fmt.Errorf("no branches found")
}

// DetectUpstreamRemote tries to find upstream remote, falls back to origin
func DetectUpstreamRemote() (string, error) {
	remotes, err := GetRemotes()
	if err != nil {
		return "", err
	}
	
	for _, remote := range remotes {
		if remote == "upstream" {
			return "upstream", nil
		}
	}
	
	for _, remote := range remotes {
		if remote == "origin" {
			return "origin", nil
		}
	}
	
	return "", fmt.Errorf("no remotes found")
}

// GetBranchInfo gets detailed info about a branch
func GetBranchInfo(branchName string, baseBranch string) (*Branch, error) {
	branch := &Branch{
		Name:   branchName,
		Status: "ok",
	}
	
	// Get description from git config
	branch.Description = GetBranchTag(branchName)
	
	// Get last commit date
	cmd := exec.Command("git", "log", "-1", "--format=%ar", branchName)
	output, err := cmd.Output()
	if err == nil {
		branch.LastCommit = strings.TrimSpace(string(output))
	}
	
	// Get ahead/behind counts
	cmd = exec.Command("git", "rev-list", "--left-right", "--count", fmt.Sprintf("%s...%s", baseBranch, branchName))
	output, err = cmd.Output()
	if err == nil {
		parts := strings.Fields(string(output))
		if len(parts) == 2 {
			fmt.Sscanf(parts[0], "%d", &branch.Behind)
			fmt.Sscanf(parts[1], "%d", &branch.Ahead)
			
			if branch.Behind > 0 {
				branch.Status = "behind"
			}
		}
	}
	
	return branch, nil
}

// FetchUpstream fetches the upstream remote
func FetchUpstream(remote string, baseBranch string) error {
	cmd := exec.Command("git", "fetch", remote, baseBranch)
	return cmd.Run()
}

// UpdateBaseBranch updates the local base branch from upstream
func UpdateBaseBranch(baseBranch string, remote string) error {
	// Check if the local base branch has diverged from the remote
	cmd := exec.Command("git", "rev-list", baseBranch, fmt.Sprintf("^%s/%s", remote, baseBranch))
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("could not check for branch divergence: %w", err)
	}
	if len(strings.TrimSpace(string(output))) > 0 {
		return fmt.Errorf("local base branch '%s' has diverged from '%s/%s'. Please resolve manually", baseBranch, remote, baseBranch)
	}

	// Checkout base branch
	cmd = exec.Command("git", "checkout", baseBranch)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to checkout %s: %w", baseBranch, err)
	}
	
	// Reset to upstream
	cmd = exec.Command("git", "reset", "--hard", fmt.Sprintf("%s/%s", remote, baseBranch))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to reset to %s/%s: %w", remote, baseBranch, err)
	}
	
	// Push to origin
	cmd = exec.Command("git", "push", "origin", baseBranch, "--force-with-lease")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to push to origin: %w", err)
	}
	
	return nil
}

// RebaseBranch rebases a branch onto the base branch
func RebaseBranch(branchName string, baseBranch string) error {
	// Checkout the branch
	cmd := exec.Command("git", "checkout", branchName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to checkout: %w", err)
	}
	
	// Rebase onto base branch
	cmd = exec.Command("git", "rebase", baseBranch)
	if err := cmd.Run(); err != nil {
		// Abort the rebase
		abortCmd := exec.Command("git", "rebase", "--abort")
		abortCmd.Run()
		return fmt.Errorf("rebase conflict")
	}
	
	return nil
}

// PushBranch pushes a branch to origin
func PushBranch(branchName string) error {
	cmd := exec.Command("git", "push", "origin", branchName, "--force-with-lease")
	return cmd.Run()
}

// DeleteLocalBranch deletes a local branch
func DeleteLocalBranch(branchName string) error {
	cmd := exec.Command("git", "branch", "-d", branchName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf(strings.TrimSpace(string(output)))
	}
	return nil
}

// DeleteRemoteBranch deletes a remote branch
func DeleteRemoteBranch(branchName string) error {
	cmd := exec.Command("git", "push", "origin", "--delete", branchName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf(strings.TrimSpace(string(output)))
	}
	return nil
}

// StashChanges stashes the current changes
func StashChanges() error {
	cmd := exec.Command("git", "stash")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf(strings.TrimSpace(string(output)))
	}
	return nil
}

// StashPop pops the latest stash
func StashPop() error {
	cmd := exec.Command("git", "stash", "pop")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf(strings.TrimSpace(string(output)))
	}
	return nil
}

// HasUncommittedChanges checks if there are uncommitted changes
func HasUncommittedChanges() bool {
	cmd := exec.Command("git", "status", "--porcelain", "-uno")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(output))) > 0
}

// GetBranchesWithInfo gets all branches with their info
func GetBranchesWithInfo(baseBranch string, excludePatterns []string) ([]*Branch, error) {
	branchNames, err := GetAllBranches()
	if err != nil {
		return nil, err
	}
	
	var branches []*Branch
	for _, name := range branchNames {
		// Skip base branch
		if name == baseBranch {
			continue
		}
		
		// Skip excluded patterns
		skip := false
		for _, pattern := range excludePatterns {
			if strings.Contains(name, pattern) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		
		branch, err := GetBranchInfo(name, baseBranch)
		if err != nil {
			continue
		}
		branches = append(branches, branch)
	}
	
	return branches, nil
}

// Sleep for a bit to show messages
func Sleep(ms int) {
	time.Sleep(time.Duration(ms) * time.Millisecond)
}
