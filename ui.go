package main

import (
	"fmt"
	"strings"
	"time"

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
	stateHelp
	stateConfirmingDelete
	stateDeleting
	stateConfirmingStash
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
	loadingDots     string // For animating the loading message
	deleteMode      bool   // Are we in deletion mode?
	selectedForActionCount int
	didStash        bool // Did we stash changes?
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

type branchDeletedMsg struct {
	branch string
	success bool
	error   string
}

type tickMsg time.Time

// InitialModel creates the initial model
func InitialModel() Model {
	return Model{
		state:   stateLoading,
		message: "Loading repository information...",
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(loadRepoInfo, tick())
}

// tick sends a tickMsg every 500ms
func tick() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// loadRepoInfo loads repository information
func loadRepoInfo() tea.Msg {
	config, err := LoadConfig()
	if err != nil {
		return errorMsg{err}
	}

	// Fetch the latest from upstream before loading branches
	if err := FetchUpstream(config.UpstreamRemote, config.BaseBranch); err != nil {
		return errorMsg{fmt.Errorf("failed to fetch upstream '%s/%s': %w", config.UpstreamRemote, config.BaseBranch, err)}
	}
	
	current, err := GetCurrentBranch()
	if err != nil {
		return errorMsg{err}
	}
	
	branches, err := GetBranchesWithInfo(config.BaseBranch, config.UpstreamRemote, config.ExcludePatterns)
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
		if m.didStash {
			StashPop()
			m.didStash = false
		}
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
		
		if m.updateIndex >= m.selectedForActionCount {
			m.state = stateDone
			if m.didStash {
				StashPop()
				m.didStash = false
			}
			return m, nil
		}
		
		// Update next branch
		return m, m.updateNextBranch()
	
	case tickMsg:
		if m.state == stateLoading {
			if len(m.loadingDots) < 3 {
				m.loadingDots += "."
			} else {
				m.loadingDots = ""
			}
			return m, tick() // Continue ticking
		}
	
	case branchDeletedMsg:
		if msg.success {
			m.successCount++
			// Mark the branch as deleted
			for _, b := range m.branches {
				if b.Name == msg.branch {
					b.Status = "deleted"
					break
				}
			}
		} else {
			m.failedBranches = append(m.failedBranches, fmt.Sprintf("%s (%s)", msg.branch, msg.error))
		}
		
		m.updateIndex++
		
		if m.updateIndex >= m.selectedForActionCount {
			m.state = stateDone
			if m.didStash {
				StashPop()
				m.didStash = false
			}
			return m, nil
		}
		
		// Delete next branch
		return m, m.deleteNextBranch()
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
	case stateConfirmingDelete:
		return m.handleConfirmingDeleteKeys(msg)
	case stateConfirmingStash:
		return m.handleConfirmingStashKeys(msg)
	case stateDone, stateError:
		if msg.String() == " " || msg.String() == "enter" {
			m.state = stateBrowsing
			m.message = ""
			m.successCount = 0
			m.failedBranches = []string{}
			m.updateIndex = 0
			m.selectedForActionCount = 0
			m.commandLog = []string{}
			m.didStash = false
			m.deleteMode = false
			for _, b := range m.branches {
				b.Selected = false
			}
			return m, nil
		} else if msg.String() == "q" || msg.String() == "ctrl+c" {
			if m.didStash {
				StashPop()
			}
			m.deleteMode = false
			return m, tea.Quit
		}
	case stateTagging:
		return m.handleTaggingKeys(msg)
	case stateHelp:
		return m.handleHelpKeys(msg)
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
	
	case "d":
		if !m.deleteMode {
			m.deleteMode = true
			m.message = "DELETE MODE: Select branches and press 'd' to confirm deletion."
		} else {
			// Check if any branches are selected for deletion
			selectedCount := 0
			for _, b := range m.branches {
				if b.Selected {
					selectedCount++
				}
			}
			if selectedCount > 0 {
				m.state = stateConfirmingDelete
				m.message = fmt.Sprintf("Ready to delete %d branch(es).", selectedCount)
			} else {
				m.message = "No branches selected for deletion."
			}
		}

	case "h":
		m.state = stateHelp
	
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
		// Clear search or exit delete mode
		if m.searchQuery != "" {
			m.searchQuery = ""
			m.cursor = 0
			return m, nil
		}
		if m.deleteMode {
			m.deleteMode = false
			m.message = ""
			// Deselect all branches when exiting delete mode
			for _, b := range m.branches {
				b.Selected = false
			}
			return m, nil
		}
	
		case "enter":
			if m.deleteMode {
				// Disable enter key in delete mode
				return m, nil
			}
			// Check for uncommitted changes before starting
			if HasUncommittedChanges() {
				m.state = stateConfirmingStash
				m.message = "You have uncommitted changes. Stash them and proceed? (y/n)"
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
	
			// Populate command log
			m.commandLog = []string{}
			m.commandLog = append(m.commandLog, fmt.Sprintf("git fetch %s %s", m.config.UpstreamRemote, m.config.BaseBranch))
			m.commandLog = append(m.commandLog, fmt.Sprintf("git checkout %s", m.config.BaseBranch))
			m.commandLog = append(m.commandLog, fmt.Sprintf("git reset --hard %s/%s", m.config.UpstreamRemote, m.config.BaseBranch))
			m.commandLog = append(m.commandLog, fmt.Sprintf("git push origin %s --force-with-lease", m.config.BaseBranch))
			for _, b := range m.branches {
				if b.Selected {
					m.commandLog = append(m.commandLog, fmt.Sprintf("git checkout %s", b.Name))
					m.commandLog = append(m.commandLog, fmt.Sprintf("git rebase %s", m.config.BaseBranch))
					m.commandLog = append(m.commandLog, fmt.Sprintf("git push origin %s --force-with-lease", b.Name))
				}
			}
	
			if manualMode {
				m.state = stateConfirming
				m.message = fmt.Sprintf("Ready to update %d branch(es). Press 'y' to continue, 'n' to cancel.", selectedCount)
			} else {
				m.state = stateUpdating
				m.updateIndex = 0
				m.successCount = 0
				m.failedBranches = []string{}
				m.selectedForActionCount = selectedCount
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
		m.selectedForActionCount = 0
		for _, b := range m.branches {
			if b.Selected {
				m.selectedForActionCount++
			}
		}
		// Populate command log
		m.commandLog = []string{}
		m.commandLog = append(m.commandLog, fmt.Sprintf("git fetch %s %s", m.config.UpstreamRemote, m.config.BaseBranch))
		m.commandLog = append(m.commandLog, fmt.Sprintf("git checkout %s", m.config.BaseBranch))
		m.commandLog = append(m.commandLog, fmt.Sprintf("git reset --hard %s/%s", m.config.UpstreamRemote, m.config.BaseBranch))
		m.commandLog = append(m.commandLog, fmt.Sprintf("git push origin %s --force-with-lease", m.config.BaseBranch))
		for _, b := range m.branches {
			if b.Selected {
				m.commandLog = append(m.commandLog, fmt.Sprintf("git checkout %s", b.Name))
				m.commandLog = append(m.commandLog, fmt.Sprintf("git rebase %s", m.config.BaseBranch))
				m.commandLog = append(m.commandLog, fmt.Sprintf("git push origin %s --force-with-lease", b.Name))
			}
		}
		return m, m.updateNextBranch()
	
	case "n", "N", "q", "ctrl+c":
		m.state = stateBrowsing
		m.message = "Update cancelled"
	}
	
	return m, nil
}

// handleConfirmingDeleteKeys handles keys in confirming delete state
func (m Model) handleConfirmingDeleteKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.state = stateDeleting
		m.updateIndex = 0
		m.successCount = 0
		m.failedBranches = []string{}
		m.selectedForActionCount = 0
		for _, b := range m.branches {
			if b.Selected {
				m.selectedForActionCount++
			}
		}

		// Populate command log for deletion
		m.commandLog = []string{}
		for _, b := range m.branches {
			if b.Selected {
				m.commandLog = append(m.commandLog, fmt.Sprintf("git branch -d %s", b.Name))
				m.commandLog = append(m.commandLog, fmt.Sprintf("git push origin --delete %s", b.Name))
			}
		}

		return m, m.deleteNextBranch()
	
	case "n", "N", "q", "ctrl+c", "esc":
		m.state = stateBrowsing
		m.deleteMode = false
		m.message = "Deletion cancelled"
		// Deselect all branches
		for _, b := range m.branches {
			b.Selected = false
		}
	}
	
	return m, nil
}

// handleConfirmingStashKeys handles keys in confirming stash state
func (m Model) handleConfirmingStashKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		if err := StashChanges(); err != nil {
			m.state = stateError
			m.error = err.Error()
			return m, nil
		}
		m.didStash = true
		
		// Populate command log
		m.commandLog = []string{}
		m.commandLog = append(m.commandLog, fmt.Sprintf("git fetch %s %s", m.config.UpstreamRemote, m.config.BaseBranch))
		m.commandLog = append(m.commandLog, fmt.Sprintf("git checkout %s", m.config.BaseBranch))
		m.commandLog = append(m.commandLog, fmt.Sprintf("git reset --hard %s/%s", m.config.UpstreamRemote, m.config.BaseBranch))
		m.commandLog = append(m.commandLog, fmt.Sprintf("git push origin %s --force-with-lease", m.config.BaseBranch))
		for _, b := range m.branches {
			if b.Selected {
				m.commandLog = append(m.commandLog, fmt.Sprintf("git checkout %s", b.Name))
				m.commandLog = append(m.commandLog, fmt.Sprintf("git rebase %s", m.config.BaseBranch))
				m.commandLog = append(m.commandLog, fmt.Sprintf("git push origin %s --force-with-lease", b.Name))
			}
		}

		// Proceed with update
		selectedCount := 0
		for _, b := range m.branches {
			if b.Selected {
				selectedCount++
			}
		}
		if selectedCount == 0 {
			m.message = "No branches selected"
			m.state = stateBrowsing
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
			m.selectedForActionCount = selectedCount
			return m, m.updateNextBranch()
		}

	case "n", "N", "q", "ctrl+c", "esc":
		m.state = stateBrowsing
		m.message = "Update cancelled."
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

// handleHelpKeys handles keys in help state
func (m Model) handleHelpKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "h", "q", "esc":
		m.state = stateBrowsing
		return m, nil
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

// deleteNextBranch deletes the next selected branch
func (m Model) deleteNextBranch() tea.Cmd {
	return func() tea.Msg {
		// Find next selected branch for deletion
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
			return branchDeletedMsg{success: false, error: "branch not found"}
		}
		
		// Delete local branch
		if err := DeleteLocalBranch(targetBranch.Name); err != nil {
			return branchDeletedMsg{branch: targetBranch.Name, success: false, error: err.Error()}
		}
		
		// Delete remote branch
		if err := DeleteRemoteBranch(targetBranch.Name); err != nil {
			return branchDeletedMsg{branch: targetBranch.Name, success: false, error: err.Error()}
		}
		
		return branchDeletedMsg{branch: targetBranch.Name, success: true}
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
	case stateConfirmingDelete:
		return m.viewConfirmingDelete()
	case stateConfirmingStash:
		return m.viewConfirmingStash()
	case stateUpdating, stateDeleting:
		return m.viewUpdating()
	case stateDone:
		return m.viewDone()
	case stateError:
		return m.viewError()
	case stateTagging:
		return m.viewTagging()
	case stateHelp:
		return m.viewHelp()
	}
	return ""
}

func (m Model) viewLoading() string {
	var s strings.Builder
	s.WriteString(titleStyle.Render("🌿 GitSync - By Hariharen"))
	s.WriteString("\n\n")
	s.WriteString(fmt.Sprintf("  %s\n  %s\n\n",
		infoStyle.Render("⏳ "+m.message+m.loadingDots),
		dimStyle.Render("Loading configuration, fetching latest from upstream, detecting current branch, and analyzing all branches...")))
	return s.String()
}

func (m Model) viewBrowsing() string {
	var s strings.Builder
	
	// Title
	if m.deleteMode {
		s.WriteString(errorStyle.Render("🔥 GitSync - Deletion Mode"))
	} else {
		s.WriteString(titleStyle.Render("🌿 GitSync - By Hariharen"))
	}
	s.WriteString("\n\n")
	
	// Config info
	baseInfo := dimStyle.Render("  Base: ") + titleStyle.Render(m.config.BaseBranch)
	remoteInfo := dimStyle.Render("  |  Remote: ") + titleStyle.Render(m.config.UpstreamRemote)
	currentInfo := dimStyle.Render("  |  Current: ") + titleStyle.Render(m.currentBranch)
	s.WriteString(lipgloss.JoinHorizontal(lipgloss.Left, baseInfo, remoteInfo, currentInfo))
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
			
			checkbox := dimStyle.Render("[ ]")
			if branch.Selected {
				checkbox = successStyle.Render("[✓]")
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
	} else if m.deleteMode {
		s.WriteString(lipgloss.JoinHorizontal(lipgloss.Left,
			dimStyle.Render("  "),
			errorStyle.Render("space"), dimStyle.Render(": select  "),
			errorStyle.Render("d"), dimStyle.Render(": confirm delete  "),
			errorStyle.Render("esc"), dimStyle.Render(": cancel"),
		))
	} else {
		s.WriteString(lipgloss.JoinHorizontal(lipgloss.Left,
			dimStyle.Render("  "),
			titleStyle.Render("↑/↓"), dimStyle.Render(": navigate  "),
			titleStyle.Render("space"), dimStyle.Render(": select  "),
			titleStyle.Render("a"), dimStyle.Render(": all  "),
			titleStyle.Render("n"), dimStyle.Render(": none  "),
			titleStyle.Render("/"), dimStyle.Render(": search  "),
			titleStyle.Render("t"), dimStyle.Render(": tag  "),
			titleStyle.Render("h"), dimStyle.Render(": help  "),
			titleStyle.Render("enter"), dimStyle.Render(": update  "),
			titleStyle.Render("d"), dimStyle.Render(": delete mode  "),
			titleStyle.Render("q"), dimStyle.Render(": quit"),
		))
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

func (m Model) viewConfirmingDelete() string {
	var s strings.Builder
	
	s.WriteString(errorStyle.Render("🔥 GitSync - Confirm Deletion"))
	s.WriteString("\n\n")
	
	s.WriteString(warningStyle.Render("  You are about to delete the following branches both locally and remotely:"))
	s.WriteString("\n\n")
	
	for _, branch := range m.branches {
		if branch.Selected {
			s.WriteString(fmt.Sprintf("    • %s\n", branch.Name))
		}
	}
	
	s.WriteString("\n")
	s.WriteString(boxStyle.Render("This action cannot be undone."))
	s.WriteString("\n\n")
	
	s.WriteString(dimStyle.Render("  Are you sure? (y/n)"))
	
	return s.String()
}

func (m Model) viewConfirmingStash() string {
	var s strings.Builder
	
	s.WriteString(warningStyle.Render("🤔 GitSync - Uncommitted Changes"))
	s.WriteString("\n\n")
	
	s.WriteString(boxStyle.Render(m.message))
	s.WriteString("\n\n")
	
	s.WriteString(dimStyle.Render("  y: stash and proceed  n: cancel"))
	
	return s.String()
}

func (m Model) viewUpdating() string {
	var s strings.Builder
	
	title := "🌿 GitSync - Updating"
	if m.state == stateDeleting {
		title = "🔥 GitSync - Deleting"
	}
	s.WriteString(titleStyle.Render(title))
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
		} else if branch.Status == "deleted" {
			icon = successStyle.Render("✓")
			status = successStyle.Render(" deleted")
		}
		
		s.WriteString(fmt.Sprintf("  %s %s%s\n", icon, branch.Name, status))
	}
	
	s.WriteString("\n")
	s.WriteString(dimStyle.Render("  Please wait..."))
	s.WriteString("\n\n")
	
	// Display predicted commands
	if len(m.commandLog) > 0 && (m.state == stateUpdating || m.state == stateDeleting) {
		s.WriteString(boxStyle.Render(
			lipgloss.JoinVertical(lipgloss.Left,
				infoStyle.Render("Commands that will be ran:"),
				dimStyle.Render(strings.Join(m.commandLog, "\n")),
			),
		))
		s.WriteString("\n")
	}
	
	return s.String()
}

func (m Model) viewDone() string {
	var s strings.Builder
	
	s.WriteString(titleStyle.Render("🌿 GitSync - Complete"))
	s.WriteString("\n\n")
	
	if m.deleteMode {
		if m.successCount > 0 {
			s.WriteString(successStyle.Render(fmt.Sprintf("  ✓ Successfully deleted %d branch(es)", m.successCount)))
			s.WriteString("\n\n")
			for _, branch := range m.branches {
				if branch.Selected && branch.Status == "deleted" {
					s.WriteString(fmt.Sprintf("    • %s\n", branch.Name))
				}
			}
		}
	} else { // stateUpdating
		if m.successCount > 0 {
			s.WriteString(successStyle.Render(fmt.Sprintf("  ✓ Successfully updated %d branch(es)", m.successCount)))
			s.WriteString("\n\n")
			
			for _, branch := range m.branches {
				if branch.Selected && branch.Status == "updated" {
					s.WriteString(fmt.Sprintf("    • %s\n", branch.Name))
				}
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
		if m.deleteMode {
			s.WriteString("    1. Manually delete the failed branches if desired.\n")
		} else {
			s.WriteString("    1. Checkout the failed branch\n")
			s.WriteString("    2. Resolve conflicts manually\n")
			s.WriteString(fmt.Sprintf("    3. Run: git rebase %s\n", m.config.BaseBranch))
			s.WriteString("    4. Push: git push origin <branch> --force-with-lease\n")
		}
	}
	
	if m.didStash {
		s.WriteString("\n")
		s.WriteString(infoStyle.Render("  ✓ Stashed changes have been restored."))
		s.WriteString("\n")
	}

	s.WriteString("\n")
	s.WriteString(dimStyle.Render("  Press space/enter to continue, q to quit"))
	
	return s.String()
}

func (m Model) viewError() string {
	var s strings.Builder
	
	s.WriteString(titleStyle.Render("🌿 GitSync - Error"))
	s.WriteString("\n\n")
	
	s.WriteString(errorStyle.Render("  ✗ " + m.error))
	s.WriteString("\n\n")

	if m.didStash {
		s.WriteString(infoStyle.Render("  ✓ Stashed changes have been restored."))
		s.WriteString("\n\n")
	}

	s.WriteString(dimStyle.Render("  Press space/enter to continue, q to quit"))
	
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

func (m Model) viewHelp() string {
	var s strings.Builder

	s.WriteString(titleStyle.Render("🌿 GitSync - Help"))
	s.WriteString("\n\n")

	s.WriteString(infoStyle.Render("What is GitSync?"))
	s.WriteString("\n")
	s.WriteString(dimStyle.Render("GitSync is a tool to streamline keeping multiple feature branches up-to-date with a central base branch (like 'main' or 'develop').\nIt provides an interactive UI to select branches and automates the process of rebasing and pushing them."))
	s.WriteString("\n\n")

	s.WriteString(infoStyle.Render("Workflow:"))
	s.WriteString("\n")
	s.WriteString(dimStyle.Render("1. On load, GitSync fetches the latest from your upstream remote to ensure all branch information is current."))
	s.WriteString("\n")
	s.WriteString(dimStyle.Render("2. It then displays a list of your local branches, showing their status relative to the base branch."))
	s.WriteString("\n")
	s.WriteString(dimStyle.Render("3. You can select one or more branches to update."))
	s.WriteString("\n")
	s.WriteString(dimStyle.Render("4. When you start the update, GitSync will first hard-reset your local base branch to match the upstream version."))
	s.WriteString("\n")
	s.WriteString(dimStyle.Render("5. It then, one-by-one, rebases each selected branch onto the updated base branch and force-pushes it to your 'origin' remote."))
	s.WriteString("\n\n")

	s.WriteString(infoStyle.Render("A Note on Safety:"))
	s.WriteString("\n")
	s.WriteString(dimStyle.Render("GitSync uses 'git push --force-with-lease'. This is a safer alternative to 'git push --force'.\nIt will not overwrite the remote branch if someone else has pushed new commits to it in the meantime, thus preventing accidental loss of work."))
	s.WriteString("\n\n")

	s.WriteString(infoStyle.Render("Commands:"))
	s.WriteString("\n")
	s.WriteString(fmt.Sprintf("  %s: navigate up\n", selectedStyle.Render("↑/k")))
	s.WriteString(fmt.Sprintf("  %s: navigate down\n", selectedStyle.Render("↓/j")))
	s.WriteString(fmt.Sprintf("  %s: select/deselect branch\n", selectedStyle.Render("space")))
	s.WriteString(fmt.Sprintf("  %s: select all visible branches\n", selectedStyle.Render("a")))
	s.WriteString(fmt.Sprintf("  %s: deselect all visible branches\n", selectedStyle.Render("n")))
	s.WriteString(fmt.Sprintf("  %s: add/edit a description for the selected branch\n", selectedStyle.Render("t")))
	s.WriteString(fmt.Sprintf("  %s: search/filter branches\n", selectedStyle.Render("/")))
	s.WriteString(fmt.Sprintf("  %s: start the update process for selected branches\n", selectedStyle.Render("enter")))
	s.WriteString(fmt.Sprintf("  %s: show this help window\n", selectedStyle.Render("h")))
	s.WriteString(fmt.Sprintf("  %s: quit the application\n", selectedStyle.Render("q/ctrl+c")))

	s.WriteString("\n")
	s.WriteString(infoStyle.Render("Status Indicators:"))
	s.WriteString("\n")
	s.WriteString(fmt.Sprintf("  %s: Branch is up-to-date with the base branch\n", successStyle.Render("●")))
	s.WriteString(fmt.Sprintf("  %s: Branch is behind the base branch and needs to be updated\n", warningStyle.Render("●")))
	s.WriteString(fmt.Sprintf("  %s: Branch has a conflict with the base branch (after a failed rebase)\n", errorStyle.Render("●")))

	s.WriteString("\n")
	s.WriteString(infoStyle.Render("Ahead/Behind Info:"))
	s.WriteString("\n")
	s.WriteString(fmt.Sprintf("  %s: Number of commits the branch is behind the base branch\n", dimStyle.Render("↓<num>")))
	s.WriteString(fmt.Sprintf("  %s: Number of commits the branch is ahead of the base branch\n", dimStyle.Render("↑<num>")))

	s.WriteString("\n\n")
	s.WriteString(dimStyle.Render("  Press h, q, or esc to return"))

	return s.String()
}
