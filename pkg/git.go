package diffstat

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type GitClient struct{}

var globalGitClient = NewGitClient()

func GetGitClient() *GitClient {
	return globalGitClient
}

func NewGitClient() *GitClient {
	return &GitClient{}
}

func (c *GitClient) ListFiles() (string, error) {
	out, err := exec.Command("git", "ls-files").Output()
	if err != nil {
		return "", fmt.Errorf("could not list files: %v", err)
	}
	return string(out), nil
}

func (c *GitClient) GetOrigin() (string, error) {
	out, err := exec.Command("git", "remote", "get-url", "origin").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("could not get origin: %v", err)
	}
	return string(out), nil
}

func (c *GitClient) CheckBranch(branch string) bool {
	_, err := exec.Command("git", "rev-parse", "--verify", branch).CombinedOutput()
	return err == nil
}

func (c *GitClient) GetRepoInfo() (string, error) {
	out, err := c.GetOrigin()
	if err != nil {
		if pwd, err := os.Getwd(); err == nil {
			return filepath.Base(pwd), nil
		}
		return "", err
	}

	remoteURL := strings.TrimSpace(string(out))
	if strings.Contains(remoteURL, "/") {
		parts := strings.Split(remoteURL, "/")
		repoName := strings.TrimSuffix(parts[len(parts)-1], ".git")
		return repoName, nil
	}

	return remoteURL, nil
}

func (c *GitClient) GetTotalLines() (int, error) {
	out, err := c.ListFiles()
	if err != nil {
		return 0, fmt.Errorf("could not list files: %v", err)
	}

	files := strings.Split(strings.TrimSpace(string(out)), "\n")
	totalLines := 0

	for _, file := range files {
		out, err := exec.Command("wc", "-l", file).Output()
		if err != nil {
			return 0, fmt.Errorf("could not count lines in file '%s': %v", file, err)
		}
		lines, err := strconv.Atoi(strings.Fields(string(out))[0])
		if err != nil {
			return 0, fmt.Errorf("could not parse line count for file '%s': %v", file, err)
		}
		totalLines += lines
	}

	return totalLines, nil
}

func (c *GitClient) GetTotalLinesWithProgress(showProgress bool) (int, error) {
	out, err := c.ListFiles()
	if err != nil {
		return 0, fmt.Errorf("could not list files: %v", err)
	}

	files := strings.Split(strings.TrimSpace(string(out)), "\n")
	totalLines := 0

	if showProgress && len(files) > 100 {
		fmt.Fprintf(os.Stderr, "Analyzing %d files...", len(files))
	}

	for i, file := range files {
		out, err := exec.Command("wc", "-l", file).Output()
		if err != nil {
			return 0, fmt.Errorf("could not count lines in file '%s': %v", file, err)
		}
		lines, err := strconv.Atoi(strings.Fields(string(out))[0])
		if err != nil {
			return 0, fmt.Errorf("could not parse line count for file '%s': %v", file, err)
		}
		totalLines += lines

		if showProgress && len(files) > 100 && i%50 == 0 {
			progress := float64(i+1) / float64(len(files)) * 100
			fmt.Fprintf(os.Stderr, "\rAnalyzing files: %.0f%%", progress)
		}
	}

	if showProgress && len(files) > 100 {
		fmt.Fprintf(os.Stderr, "\rAnalyzing files: 100%%\n")
	}

	return totalLines, nil
}

func (c *GitClient) GetChangedLines(branch1, branch2 string) (int, error) {
	out, err := exec.Command("git", "diff", "--stat", branch1, branch2).Output()
	if err != nil {
		return 0, fmt.Errorf("git diff failed: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 {
		return 0, nil
	}
	lastLine := lines[len(lines)-1]

	fields := strings.Fields(lastLine)
	if len(fields) < 1 {
		return 0, nil
	}

	changedLines, err := strconv.Atoi(fields[0])
	if err != nil {
		return 0, err
	}

	return changedLines, nil
}
