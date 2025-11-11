package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Styles
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			MarginBottom(1)

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("170")).
			Bold(true)

	normalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")).
			Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)

	warningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Bold(true)

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39"))

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2)
)

// Model states
type state int

const (
	stateLoading state = iota
	stateBrowsing
	stateConfirming
	stateUpdating
	stateDone
	stateError
	stateTagging
)

// Model represents the application state
type Model struct {
	state           state
	config          *Config
	branches        []*Branch
	cursor          int
	message         string
	error           string
	currentBranch   string
	originalBranch  string
	updateIndex     int
	successCount    int
	failedBranches  []string
	tagInput        string
	tagMode         bool
	commandLog      []string
	searchMode      bool
	searchQuery     string
}

// Messages
type loadedMsg struct {
	branches []*Branch
	config   *Config
	current  string
}

type errorMsg struct {
	err error
}

type updateCompleteMsg struct{}

type branchUpdatedMsg struct {
	branch string
	success bool
	error   string
}

// InitialModel creates the initial model
func InitialModel() Model {
	return Model{
		state:   stateLoading,
		message: "Loading repository information...",
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return loadRepoInfo
}

// loadRepoInfo loads repository information
func loadRepoInfo() tea.Msg {
	config := LoadConfig()
	
	current, err := GetCurrentBranch()
	if err != nil {
		return errorMsg{err}
	}
	
	branches, err := GetBranchesWithInfo(config.BaseBranch, config.ExcludePatterns)
	if err != nil {
		return errorMsg{err}
	}
	
	return loadedMsg{
		branches: branches,
		config:   config,
		current:  current,
	}
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)
	
	case loadedMsg:
		m.branches = msg.branches
		m.config = msg.config
		m.currentBranch = msg.current
		m.originalBranch = msg.current
		m.state = stateBrowsing
		m.message = ""
		return m, nil
	
	case errorMsg:
		m.state = stateError
		m.error = msg.err.Error()
		return m, nil
	
	case branchUpdatedMsg:
		if msg.success {
			m.successCount++
			// Update branch status
			for _, b := range m.branches {
				if b.Name == msg.branch {
					b.Status = "updated"
					break
				}
			}
		} else {
			m.failedBranches = append(m.failedBranches, fmt.Sprintf("%s (%s)", msg.branch, msg.error))
		}
		
		m.updateIndex++
		
		// Check if we're done
		selectedCount := 0
		for _, b := range m.branches {
			if b.Selected {
				selectedCount++
			}
		}
		
		if m.updateIndex >= selectedCount {
			m.state = stateDone
			return m, nil
		}
		
		// Update next branch
		return m, m.updateNextBranch()
	}
	
	return m, nil
}

// handleKeyPress handles keyboard input
func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.state {
	case stateBrowsing:
		return m.handleBrowsingKeys(msg)
	case stateConfirming:
		return m.handleConfirmingKeys(msg)
	case stateDone, stateError:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case stateTagging:
		return m.handleTaggingKeys(msg)
	}
	
	return m, nil
}

// handleBrowsingKeys handles keys in browsing state
func (m Model) handleBrowsingKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// If in search mode, handle search input
	if m.searchMode {
		return m.handleSearchKeys(msg)
	}
	
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	
	case "up", "k":
		filtered := m.getFilteredBranches()
		if m.cursor > 0 {
			m.cursor--
		}
		// Ensure cursor stays within filtered list
		if m.cursor >= len(filtered) && len(filtered) > 0 {
			m.cursor = len(filtered) - 1
		}
	
	case "down", "j":
		filtered := m.getFilteredBranches()
		if m.cursor < len(filtered)-1 {
			m.cursor++
		}
	
	case " ":
		filtered := m.getFilteredBranches()
		if m.cursor < len(filtered) {
			filtered[m.cursor].Selected = !filtered[m.cursor].Selected
		}
	
	case "a":
		// Select all visible (filtered) branches
		filtered := m.getFilteredBranches()
		for _, b := range filtered {
			b.Selected = true
		}
	
	case "n":
		// Deselect all visible (filtered) branches
		filtered := m.getFilteredBranches()
		for _, b := range filtered {
			b.Selected = false
		}
	
	case "t":
		// Tag current branch
		filtered := m.getFilteredBranches()
		if m.cursor < len(filtered) {
			m.state = stateTagging
			m.tagInput = filtered[m.cursor].Description
		}
	
	case "/":
		// Enter search mode
		m.searchMode = true
		m.searchQuery = ""
		m.cursor = 0
		return m, nil
	
	case "esc":
		// Clear search
		if m.searchQuery != "" {
			m.searchQuery = ""
			m.cursor = 0
			return m, nil
		}
	
		case "enter":
			// Check for uncommitted changes before starting
			if HasUncommittedChanges() {
				m.message = "Aborting: You have uncommitted changes. Please stash or commit them first."
				return m, nil
			}
	
			// Start update process
			selectedCount := 0
			for _, b := range m.branches {
				if b.Selected {
					selectedCount++
				}
			}
	
			if selectedCount == 0 {
				m.message = "No branches selected"
				return m, nil
			}
	
			if manualMode {
				m.state = stateConfirming
				m.message = fmt.Sprintf("Ready to update %d branch(es). Press 'y' to continue, 'n' to cancel.", selectedCount)
			} else {
				m.state = stateUpdating
				m.updateIndex = 0
				m.successCount = 0
				m.failedBranches = []string{}
				m.commandLog = []string{}
				return m, m.updateNextBranch()
			}	}
	
	return m, nil
}

// handleSearchKeys handles keys in search mode
func (m Model) handleSearchKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "esc":
		// Exit search mode
		m.searchMode = false
		return m, nil
	
	case "backspace":
		if len(m.searchQuery) > 0 {
			m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
			m.cursor = 0 // Reset cursor when search changes
		}
	
	case "ctrl+c":
		return m, tea.Quit
	
	default:
		// Add character to search query
		if len(msg.String()) == 1 {
			m.searchQuery += msg.String()
			m.cursor = 0 // Reset cursor when search changes
		}
	}
	
	return m, nil
}

// getFilteredBranches returns branches that match the search query
func (m Model) getFilteredBranches() []*Branch {
	if m.searchQuery == "" {
		return m.branches
	}
	
	var filtered []*Branch
	query := strings.ToLower(m.searchQuery)
	
	for _, branch := range m.branches {
		// Search in branch name and description
		if strings.Contains(strings.ToLower(branch.Name), query) ||
			strings.Contains(strings.ToLower(branch.Description), query) {
			filtered = append(filtered, branch)
		}
	}
	
	return filtered
}

// handleConfirmingKeys handles keys in confirming state
func (m Model) handleConfirmingKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.state = stateUpdating
		m.updateIndex = 0
		m.successCount = 0
		m.failedBranches = []string{}
		return m, m.updateNextBranch()
	
	case "n", "N", "q", "ctrl+c":
		m.state = stateBrowsing
		m.message = "Update cancelled"
	}
	
	return m, nil
}

// handleTaggingKeys handles keys in tagging state
func (m Model) handleTaggingKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		// Save tag
		if m.cursor < len(m.branches) {
			branch := m.branches[m.cursor]
			if m.tagInput != "" {
				SetBranchTag(branch.Name, m.tagInput)
				branch.Description = m.tagInput
			} else {
				RemoveBranchTag(branch.Name)
				branch.Description = ""
			}
		}
		m.state = stateBrowsing
		m.tagInput = ""
	
	case "esc", "ctrl+c":
		m.state = stateBrowsing
		m.tagInput = ""
	
	case "backspace":
		if len(m.tagInput) > 0 {
			m.tagInput = m.tagInput[:len(m.tagInput)-1]
		}
	
	default:
		// Add character to input
		if len(msg.String()) == 1 {
			m.tagInput += msg.String()
		}
	}
	
	return m, nil
}

// updateNextBranch updates the next selected branch
func (m Model) updateNextBranch() tea.Cmd {
	return func() tea.Msg {
		// Find next selected branch
		var targetBranch *Branch
		currentIndex := 0
		for _, b := range m.branches {
			if b.Selected {
				if currentIndex == m.updateIndex {
					targetBranch = b
					break
				}
				currentIndex++
			}
		}
		
		if targetBranch == nil {
			return branchUpdatedMsg{success: false, error: "branch not found"}
		}
		
		// Update base branch first (only on first iteration)
		if m.updateIndex == 0 {
			if err := FetchUpstream(m.config.UpstreamRemote, m.config.BaseBranch); err != nil {
				return branchUpdatedMsg{branch: targetBranch.Name, success: false, error: "fetch failed"}
			}
			
			if err := UpdateBaseBranch(m.config.BaseBranch, m.config.UpstreamRemote); err != nil {
				return branchUpdatedMsg{branch: targetBranch.Name, success: false, error: err.Error()}
			}
		}
		
		// Rebase the branch
		if err := RebaseBranch(targetBranch.Name, m.config.BaseBranch); err != nil {
			return branchUpdatedMsg{branch: targetBranch.Name, success: false, error: err.Error()}
		}
		
		// Push the branch
		if err := PushBranch(targetBranch.Name); err != nil {
			return branchUpdatedMsg{branch: targetBranch.Name, success: false, error: "push failed"}
		}
		
		return branchUpdatedMsg{branch: targetBranch.Name, success: true}
	}
}

// View renders the UI
func (m Model) View() string {
	switch m.state {
	case stateLoading:
		return m.viewLoading()
	case stateBrowsing:
		return m.viewBrowsing()
	case stateConfirming:
		return m.viewConfirming()
	case stateUpdating:
		return m.viewUpdating()
	case stateDone:
		return m.viewDone()
	case stateError:
		return m.viewError()
	case stateTagging:
		return m.viewTagging()
	}
	return ""
}

func (m Model) viewLoading() string {
	return fmt.Sprintf("\n  %s\n\n", infoStyle.Render("⏳ "+m.message))
}

func (m Model) viewBrowsing() string {
	var s strings.Builder
	
	// Title
	s.WriteString(titleStyle.Render("🌿 GitSync - By Hariharen"))
	s.WriteString("\n\n")
	
	// Config info
	s.WriteString(dimStyle.Render(fmt.Sprintf("  Base: %s  |  Remote: %s  |  Current: %s",
		m.config.BaseBranch, m.config.UpstreamRemote, m.currentBranch)))
	s.WriteString("\n\n")
	
	// Search bar
	if m.searchMode {
		s.WriteString(infoStyle.Render("  Search: "))
		s.WriteString(selectedStyle.Render(m.searchQuery + "█"))
		s.WriteString(dimStyle.Render(" (enter/esc to exit)"))
		s.WriteString("\n\n")
	} else if m.searchQuery != "" {
		s.WriteString(infoStyle.Render(fmt.Sprintf("  Filtered by: %s ", m.searchQuery)))
		s.WriteString(dimStyle.Render("(esc to clear, / to edit)"))
		s.WriteString("\n\n")
	}
	
	// Get filtered branches
	filteredBranches := m.getFilteredBranches()
	
	// Branch list
	if len(m.branches) == 0 {
		s.WriteString(warningStyle.Render("  No branches found (excluding base branch)"))
	} else if len(filteredBranches) == 0 {
		s.WriteString(warningStyle.Render(fmt.Sprintf("  No branches match '%s'", m.searchQuery)))
	} else {
		// Show count if filtered
		if m.searchQuery != "" {
			s.WriteString(dimStyle.Render(fmt.Sprintf("  Showing %d of %d branches", len(filteredBranches), len(m.branches))))
			s.WriteString("\n\n")
		}
		
		for i, branch := range filteredBranches {
			cursor := "  "
			if i == m.cursor {
				cursor = "❯ "
			}
			
			checkbox := "☐"
			if branch.Selected {
				checkbox = "☑"
			}
			
			// Status indicator
			statusIcon := "●"
			statusColor := lipgloss.Color("42") // green
			if branch.Status == "behind" {
				statusIcon = "●"
				statusColor = lipgloss.Color("214") // yellow
			} else if branch.Status == "conflict" {
				statusIcon = "●"
				statusColor = lipgloss.Color("196") // red
			}
			
			status := lipgloss.NewStyle().Foreground(statusColor).Render(statusIcon)
			
			// Branch name - highlight search match
			name := branch.Name
			if m.searchQuery != "" && strings.Contains(strings.ToLower(name), strings.ToLower(m.searchQuery)) {
				// Highlight the matching part
				lowerName := strings.ToLower(name)
				lowerQuery := strings.ToLower(m.searchQuery)
				idx := strings.Index(lowerName, lowerQuery)
				if idx >= 0 {
					before := name[:idx]
					match := name[idx : idx+len(m.searchQuery)]
					after := name[idx+len(m.searchQuery):]
					name = before + warningStyle.Render(match) + after
				}
			}
			
			if i == m.cursor {
				name = selectedStyle.Render(name)
			} else {
				name = normalStyle.Render(name)
			}
			
			// Behind/ahead info
			behindAhead := ""
			if branch.Behind > 0 || branch.Ahead > 0 {
				behindAhead = dimStyle.Render(fmt.Sprintf(" ↓%d ↑%d", branch.Behind, branch.Ahead))
			}
			
			// Description
			desc := ""
			if branch.Description != "" {
				desc = dimStyle.Render(fmt.Sprintf(" - %s", branch.Description))
			}
			
			// Last commit
			lastCommit := ""
			if branch.LastCommit != "" {
				lastCommit = dimStyle.Render(fmt.Sprintf(" (%s)", branch.LastCommit))
			}
			
			line := fmt.Sprintf("%s%s %s %s%s%s%s",
				cursor, checkbox, status, name, behindAhead, desc, lastCommit)
			
			s.WriteString(line)
			s.WriteString("\n")
		}
	}
	
	s.WriteString("\n")
	
	// Message
	if m.message != "" {
		s.WriteString(warningStyle.Render("  " + m.message))
		s.WriteString("\n\n")
	}
	
	// Help
	if m.searchMode {
		s.WriteString(dimStyle.Render("  Type to search  enter/esc: exit search"))
	} else {
		s.WriteString(dimStyle.Render("  ↑/↓: navigate  space: select  a: all  n: none  /: search  t: tag  enter: update  q: quit"))
	}
	
	return s.String()
}

func (m Model) viewConfirming() string {
	var s strings.Builder
	
	s.WriteString(titleStyle.Render("🌿 GitSync - Confirmation"))
	s.WriteString("\n\n")
	
	s.WriteString(boxStyle.Render(m.message))
	s.WriteString("\n\n")
	
	s.WriteString(infoStyle.Render("  Selected branches:"))
	s.WriteString("\n")
	for _, branch := range m.branches {
		if branch.Selected {
			s.WriteString(fmt.Sprintf("    • %s", branch.Name))
			if branch.Description != "" {
				s.WriteString(dimStyle.Render(fmt.Sprintf(" - %s", branch.Description)))
			}
			s.WriteString("\n")
		}
	}
	
	s.WriteString("\n")
	s.WriteString(infoStyle.Render("  Operations:"))
	s.WriteString("\n")
	s.WriteString(fmt.Sprintf("    1. Fetch %s/%s\n", m.config.UpstreamRemote, m.config.BaseBranch))
	s.WriteString(fmt.Sprintf("    2. Update local %s\n", m.config.BaseBranch))
	s.WriteString(fmt.Sprintf("    3. Rebase each branch onto %s\n", m.config.BaseBranch))
	s.WriteString("    4. Push each branch to origin\n")
	
	s.WriteString("\n")
	s.WriteString(dimStyle.Render("  y: confirm  n: cancel"))
	
	return s.String()
}

func (m Model) viewUpdating() string {
	var s strings.Builder
	
	s.WriteString(titleStyle.Render("🌿 GitSync - Updating"))
	s.WriteString("\n\n")
	
	totalSelected := 0
	for _, b := range m.branches {
		if b.Selected {
			totalSelected++
		}
	}
	
	progress := fmt.Sprintf("Progress: %d/%d", m.updateIndex+1, totalSelected)
	s.WriteString(infoStyle.Render("  " + progress))
	s.WriteString("\n\n")
	
	// Show branch statuses
	for _, branch := range m.branches {
		if !branch.Selected {
			continue
		}
		
		icon := dimStyle.Render("○")
		status := ""
		
		if branch.Status == "updated" {
			icon = successStyle.Render("✓")
			status = successStyle.Render(" updated")
		}
		
		s.WriteString(fmt.Sprintf("  %s %s%s\n", icon, branch.Name, status))
	}
	
	s.WriteString("\n")
	s.WriteString(dimStyle.Render("  Please wait..."))
	
	return s.String()
}

func (m Model) viewDone() string {
	var s strings.Builder
	
	s.WriteString(titleStyle.Render("🌿 GitSync - Complete"))
	s.WriteString("\n\n")
	
	if m.successCount > 0 {
		s.WriteString(successStyle.Render(fmt.Sprintf("  ✓ Successfully updated %d branch(es)", m.successCount)))
		s.WriteString("\n\n")
		
		for _, branch := range m.branches {
			if branch.Selected && branch.Status == "updated" {
				s.WriteString(fmt.Sprintf("    • %s\n", branch.Name))
			}
		}
	}
	
	if len(m.failedBranches) > 0 {
		s.WriteString("\n")
		s.WriteString(errorStyle.Render(fmt.Sprintf("  ✗ Failed: %d branch(es)", len(m.failedBranches))))
		s.WriteString("\n\n")
		
		for _, failed := range m.failedBranches {
			s.WriteString(fmt.Sprintf("    • %s\n", failed))
		}
		
		s.WriteString("\n")
		s.WriteString(warningStyle.Render("  Next steps:"))
		s.WriteString("\n")
		s.WriteString("    1. Checkout the failed branch\n")
		s.WriteString("    2. Resolve conflicts manually\n")
		s.WriteString(fmt.Sprintf("    3. Run: git rebase %s\n", m.config.BaseBranch))
		s.WriteString("    4. Push: git push origin <branch> --force-with-lease\n")
	}
	
	s.WriteString("\n")
	s.WriteString(dimStyle.Render("  Press q to quit"))
	
	return s.String()
}

func (m Model) viewError() string {
	var s strings.Builder
	
	s.WriteString(titleStyle.Render("🌿 GitSync - Error"))
	s.WriteString("\n\n")
	
	s.WriteString(errorStyle.Render("  ✗ " + m.error))
	s.WriteString("\n\n")
	s.WriteString(dimStyle.Render("  Press q to quit"))
	
	return s.String()
}

func (m Model) viewTagging() string {
	var s strings.Builder
	
	s.WriteString(titleStyle.Render("🌿 GitSync - Tag Branch"))
	s.WriteString("\n\n")
	
	if m.cursor < len(m.branches) {
		branch := m.branches[m.cursor]
		s.WriteString(infoStyle.Render(fmt.Sprintf("  Branch: %s", branch.Name)))
		s.WriteString("\n\n")
		
		s.WriteString("  Description: ")
		s.WriteString(selectedStyle.Render(m.tagInput + "█"))
		s.WriteString("\n\n")
		
		s.WriteString(dimStyle.Render("  enter: save  esc: cancel"))
	}
	
	return s.String()
}
