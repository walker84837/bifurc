package diffstat

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// GitClient is a simple wrapper around running git commands with exec.Command().
type GitClient struct {
	// Dir is the directory where commands are run
	Dir string
}

var globalGitClient = NewGitClient()

// GetGitClient returns a reference to the global git client.
func GetGitClient() *GitClient {
	return globalGitClient
}

// NewGitClient returns a new instance of a GitClient.
func NewGitClient() *GitClient {
	return &GitClient{}
}

func (c *GitClient) gitCmd(args ...string) *exec.Cmd {
	cmd := exec.Command("git", args...)
	if c.Dir != "" {
		cmd.Dir = c.Dir
	}
	return cmd
}

// GetOrigin returns the repository's "origin" remote, or an error if the command
// fails.
func (c *GitClient) GetOrigin() (string, error) {
	out, err := c.gitCmd("remote", "get-url", "origin").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("could not get origin: %v", err)
	}
	return string(out), nil
}

// CheckBranch returns true if the given branch (or ref) exists in the repository.
func (c *GitClient) CheckBranch(branch string) bool {
	err := c.gitCmd("rev-parse", "--verify", branch).Run()
	return err == nil
}

// GetRepoInfo returns the repository name from the Git origin URL (falls back to
// current directory name if origin is unavailable).
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

// GetEmptyTreeHash returns the SHA-1 hash of an empty Git tree, or an error if the command fails.
func (c *GitClient) GetEmptyTreeHash() (string, error) {
	cmd := c.gitCmd("hash-object", "-t", "tree", "--stdin")
	cmd.Stdin = bytes.NewReader([]byte{})
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

// GetTextDiff returns the total number of changed text lines and a list of binary files changed
// between two git revisions b1 and b2, or an error.
func (c *GitClient) GetTextDiff(b1, b2 string) (deltaLines int, changedBinaryFiles []string, err error) {
	out, err := c.gitCmd("diff", "--numstat", b1, b2).Output()
	if err != nil {
		return 0, nil, fmt.Errorf("git diff failed: %v", err)
	}

	output := strings.TrimSpace(string(out))
	if output == "" {
		return 0, nil, nil
	}

	for _, line := range strings.Split(output, "\n") {
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 3 {
			continue
		}

		additions, deletions, path := parts[0], parts[1], parts[2]

		if additions == "-" || deletions == "-" {
			changedBinaryFiles = append(changedBinaryFiles, path)
		} else {
			add, _ := strconv.Atoi(additions)
			del, _ := strconv.Atoi(deletions)
			deltaLines += add + del
		}
	}

	return deltaLines, changedBinaryFiles, nil
}

func (c *GitClient) getBlobSize(branch, file string) (int64, error) {
	out, err := c.gitCmd("cat-file", "-s", fmt.Sprintf("%s:%s", branch, file)).Output()
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
}

// GetBinaryByteDelta computes the total absolute byte-size difference across the given files
// between two Git object references (b1 and b2).
func (c *GitClient) GetBinaryByteDelta(b1, b2 string, files []string) (int64, error) {
	var delta int64
	for _, file := range files {
		s1, err1 := c.getBlobSize(b1, file)
		s2, err2 := c.getBlobSize(b2, file)

		var size1, size2 int64
		if err1 == nil {
			size1 = s1
		}
		if err2 == nil {
			size2 = s2
		}

		diff := size1 - size2
		if diff < 0 {
			diff = -diff
		}
		delta += diff
	}
	return delta, nil
}

type branchStats struct {
	totalLoc        int
	totalBinarySize int64
}

func (c *GitClient) parseLsTreeSizeMap(branch string) (map[string]int64, error) {
	out, err := c.gitCmd("ls-tree", "-r", "-l", branch).Output()
	if err != nil {
		return nil, fmt.Errorf("could not list files on %s: %v", branch, err)
	}

	sizeMap := make(map[string]int64)
	for _, line := range strings.Split(string(strings.TrimSpace(string(out))), "\n") {
		if line == "" {
			continue
		}
		tabIdx := strings.Index(line, "\t")
		if tabIdx < 0 {
			continue
		}
		meta := line[:tabIdx]
		path := line[tabIdx+1:]

		metaParts := strings.Fields(meta)
		if len(metaParts) < 4 {
			continue
		}
		size, _ := strconv.ParseInt(metaParts[3], 10, 64)
		sizeMap[path] = size
	}
	return sizeMap, nil
}

// GetRepoStats computes repository statistics for a branch: it returns total non-binary lines
// of code (totalLoc) and total size of binary files in bytes (totalBinarySize). Returns an error on failure.
func (c *GitClient) GetRepoStats(branch string) (totalLoc int, totalBinarySize int64, err error) {
	emptyTree, err := c.GetEmptyTreeHash()
	if err != nil {
		return 0, 0, fmt.Errorf("could not create empty tree: %v", err)
	}

	out, err := c.gitCmd("diff", "--numstat", emptyTree, branch).Output()
	if err != nil {
		return 0, 0, fmt.Errorf("could not get stats for %s: %v", branch, err)
	}

	output := strings.TrimSpace(string(out))
	if output == "" {
		return 0, 0, nil
	}

	sizeMap, err := c.parseLsTreeSizeMap(branch)
	if err != nil {
		return 0, 0, err
	}

	for _, line := range strings.Split(output, "\n") {
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 3 {
			continue
		}

		additions, deletions, path := parts[0], parts[1], parts[2]

		if additions == "-" || deletions == "-" {
			if size, ok := sizeMap[path]; ok {
				totalBinarySize += size
			}
		} else {
			add, _ := strconv.Atoi(additions)
			totalLoc += add
		}
	}

	return totalLoc, totalBinarySize, nil
}
