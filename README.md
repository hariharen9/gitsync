# 🌿 GitSync - Smart Branch Manager

A beautiful, interactive TUI (Terminal User Interface) tool for managing multiple git branches with ease. It transforms the tedious task of updating multiple branches into a visual, enjoyable experience.

## The Problem It Solves

When working with multiple feature branches, you often need to:
- Keep them all up-to-date with the main branch
- Rebase them regularly to avoid conflicts
- Remember what each branch is for
- See which branches are behind

Doing this manually for each branch is tedious and error-prone. GitSync provides a visual and interactive solution to streamline this workflow.

## ✨ Core Features

### 🔍 Smart Auto-Detection
- **Base Branch Detection** - Automatically finds your main branch (main, master, dev-integration, develop).
- **Remote Detection** - Discovers upstream and origin remotes without configuration.
- **Branch Discovery** - Scans and lists all local branches (excluding the base branch).
- **Zero Config** - Works out of the box for standard git workflows.

### 📊 Rich Branch Information
- **Ahead/Behind Counts** - Shows how many commits each branch is ahead or behind the base.
- **Last Commit Time** - Displays when each branch was last updated (e.g., "2 days ago").
- **Status Indicators** - Visual dots showing branch state:
  - 🟢 Green: Up to date with base.
  - 🟡 Yellow: Behind base branch.
  - 🔴 Red: Has conflicts.
- **Custom Descriptions** - Add notes to remember what each branch is for.

### 🎯 Interactive Selection
- **Keyboard Navigation** - Use arrow keys (↑/↓) or vim-style (j/k).
- **Toggle Selection** - Use the spacebar to select/deselect individual branches.
- **Bulk Operations**:
  - Press `a` to select all branches.
  - Press `n` to deselect all branches.
- **Visual Feedback** - Selected branches show checkboxes (☑).

### 📝 Branch Tagging System
- **Add Descriptions** - Press `t` to tag any branch with a description.
- **Persistent Storage** - Tags are stored in your local git config and persist across sessions.
- **Easy Editing** - A simple text input interface for adding and editing tags.

### 🔄 Smart Update Process
1. **Fetch Upstream** - Downloads the latest changes from your upstream remote.
2. **Update Base Branch** - Resets your local base branch to match upstream.
3. **Rebase Branches** - Rebases each selected branch onto the updated base.
4. **Push to Origin** - Force-pushes (with lease) the rebased branches to your origin.
5. **Conflict Handling** - Gracefully skips branches with conflicts and reports them at the end.

### 🎨 Beautiful & Informative UI
- **Color-Coded Interface** for quick status recognition.
- **Real-Time Updates** with a live progress bar during operations.
- **Detailed Summary Report** showing successful and failed operations.

### 🛡️ Safe Operations
- **Force-with-Lease** - Uses `--force-with-lease` for safer force pushing.
- **Conflict Detection** - Detects and skips conflicting branches, never leaving the repository in a broken state.
- **Uncommitted Changes Check** - Warns you if you have uncommitted work before starting.

## 🚀 Installation

You can build from source and install the `gitsync` binary using the provided scripts or Makefile.

1.  **Clone the repository:**
    ```bash
    git clone <repository_url>
    cd gitsync
    ```

2.  **Build the binary:**
    Using the build script:
    ```bash
    ./scripts/build.sh
    ```
    Or using the Makefile:
    ```bash
    make build
    ```

3.  **Install it globally (optional but recommended):**
    Using the install script:
    ```bash
    ./scripts/install.sh
    ```
    Or using the Makefile:
    ```bash
    make install
    ```
    The install script will place the `gitsync` binary in `/usr/local/bin/` (or another suitable location) and handle necessary permissions.


## 📖 Getting Started: 5-Minute Tutorial

1.  **Navigate to your git repository:**
    ```bash
    cd /path/to/your/repo
    ```

2.  **Run GitSync:**
    ```bash
    gitsync
    ```
    You'll see a list of all your local branches.

3.  **Browse and Select:**
    - Use the **arrow keys** (`↑`/`↓`) to navigate.
    - Press the **spacebar** to select one or more branches.
    - Press `a` to select all branches.

4.  **Tag a Branch (Optional):**
    - Navigate to a branch and press `t`.
    - Type a description (e.g., "Feature: new payment gateway") and press `Enter`. This helps you remember the purpose of the branch.

5.  **Update:**
    - Press `Enter` to start the update process.
    - GitSync will fetch, update your base branch, and rebase all selected branches.

6.  **Review the Summary:**
    - After the process completes, a summary will show which branches were updated successfully and which failed (e.g., due to conflicts).

## ⌨️ Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `↑` / `↓` | Navigate up/down |
| `j` / `k` | Navigate (vim-style) |
| `space` | Toggle selection |
| `a` | Select all branches |
| `n` | Deselect all branches |
| `t` | Tag/describe branch |
| `enter` | Start update process |
| `y` | Confirm (in manual mode) |
| `n` | Cancel (in manual mode) |
| `esc` | Cancel tagging |
| `q` | Quit application |
| `ctrl+c` | Force quit |

## ⚙️ Configuration (Optional)

For most standard workflows, no configuration is needed. However, you can customize GitSync's behavior by creating a `.gitsync.yaml` file in your repository's root directory.

```yaml
# .gitsync.yaml

# Override the auto-detected base branch
base_branch: dev-integration

# Specify non-standard remote names
upstream_remote: upstream
origin_remote: fork

# Configure patterns to exclude certain branches from the list
exclude_patterns:
  - "release/"
  - "hotfix/"
  - "archive/"
```

## 🎛️ Manual Mode

If you prefer more control or want to see what commands are being run, use the manual mode flag:

```bash
gitsync -m
# or
gitsync --manual
```

In manual mode, GitSync will show you exactly what will happen and ask for your confirmation before starting the update process.

## 🛠️ For Developers: Technical Details

### Project Structure
```
gitsync/
├── src/                       # All Go source files
│   ├── main.go                # Entry point, CLI flags
│   ├── git.go                 # Git operations (fetch, rebase, push, etc.)
│   ├── ui.go                  # Bubbletea UI (model, update, view)
│   ├── config.go              # Configuration loading/saving
│   └── tags.go                # Branch tagging system
├── scripts/                   # Shell scripts for build and install
│   ├── build.sh               # Cross-platform build script
│   └── install.sh             # Installation script
├── go.mod                     # Go module definition
├── go.sum                     # Dependency checksums
├── Makefile                   # Build automation
├── README.md                  # This file
├── .gitignore
├── .gitsync.yaml.example
└── LICENSE
```

### Dependencies
- **Bubbletea**: TUI framework
- **Lipgloss**: Terminal styling
- **YAML v3**: Configuration parsing

### Building from Source
- **Build for your platform:**
  ```bash
  ./scripts/build.sh
  ```
- **Run tests or other commands:**
  ```bash
  make build    # Build the binary
  make install  # Install it globally
  make clean    # Clean build artifacts
  ```
- **Cross-platform builds:**
  The `scripts/build.sh` script is configured to build for multiple platforms (macOS, Linux, Windows).
  ```bash
  ./scripts/build.sh all
  ```

## License

This project is licensed under the MIT License.