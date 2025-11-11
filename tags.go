package main

import (
	"os/exec"
	"strings"
)

// GetBranchTag gets the description tag for a branch from git config
func GetBranchTag(branchName string) string {
	cmd := exec.Command("git", "config", "branch."+branchName+".description")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// SetBranchTag sets the description tag for a branch in git config
func SetBranchTag(branchName string, description string) error {
	cmd := exec.Command("git", "config", "branch."+branchName+".description", description)
	return cmd.Run()
}

// RemoveBranchTag removes the description tag for a branch
func RemoveBranchTag(branchName string) error {
	cmd := exec.Command("git", "config", "--unset", "branch."+branchName+".description")
	return cmd.Run()
}
