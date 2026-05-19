package diffstat

import (
	"os"
	"os/exec"
	"testing"
)

func setupTestRepo(t *testing.T, c *GitClient) {
	t.Helper()
	dir := c.Dir

	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
		}
	}

	run("init")
	run("checkout", "-b", "main")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test")

	writeFile(t, dir+"/a.txt", "line1\nline2\nline3\n")
	writeFile(t, dir+"/b.txt", "line1\nline2\n")
	writeBinary(t, dir+"/image.png", 100)
	run("add", ".")
	run("commit", "-m", "initial")

	run("checkout", "-b", "feature")
	writeFile(t, dir+"/a.txt", "line1\nline2\nline3\nline4\nline5\n")
	writeFile(t, dir+"/c.txt", "new line\n")
	os.Remove(dir + "/b.txt")
	writeBinary(t, dir+"/image.png", 120)
	writeBinary(t, dir+"/data.bin", 50)
	run("add", "-A")
	run("commit", "-m", "feature work")

	run("checkout", "main")
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
}

func writeBinary(t *testing.T, path string, size int) {
	t.Helper()
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i % 256)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
}

func TestCheckBranch(t *testing.T) {
	c := &GitClient{Dir: t.TempDir()}
	setupTestRepo(t, c)

	if !c.CheckBranch("main") {
		t.Error("expected 'main' branch to exist")
	}
	if !c.CheckBranch("feature") {
		t.Error("expected 'feature' branch to exist")
	}
	if c.CheckBranch("nonexistent") {
		t.Error("expected 'nonexistent' branch to not exist")
	}
}

func TestGetEmptyTreeHash(t *testing.T) {
	c := &GitClient{Dir: t.TempDir()}
	setupTestRepo(t, c)

	hash, err := c.GetEmptyTreeHash()
	if err != nil {
		t.Fatalf("GetEmptyTreeHash failed: %v", err)
	}
	if len(hash) != 40 {
		t.Errorf("empty tree hash length = %d, want 40 (got %q)", len(hash), hash)
	}
}

func TestGetTextDiff_SameBranch(t *testing.T) {
	c := &GitClient{Dir: t.TempDir()}
	setupTestRepo(t, c)

	deltaLines, binaryFiles, err := c.GetTextDiff("main", "main")
	if err != nil {
		t.Fatalf("GetTextDiff failed: %v", err)
	}
	if deltaLines != 0 {
		t.Errorf("same branch deltaLines = %d, want 0", deltaLines)
	}
	if len(binaryFiles) != 0 {
		t.Errorf("same branch binaryFiles = %v, want empty", binaryFiles)
	}
}

func TestGetTextDiff_DifferentBranches(t *testing.T) {
	c := &GitClient{Dir: t.TempDir()}
	setupTestRepo(t, c)

	deltaLines, binaryFiles, err := c.GetTextDiff("main", "feature")
	if err != nil {
		t.Fatalf("GetTextDiff failed: %v", err)
	}

	// a.txt: +2 lines, c.txt: +1 line, b.txt: -2 lines = total 5
	if deltaLines != 5 {
		t.Errorf("deltaLines = %d, want 5 (a.txt+2 + c.txt+1 + b.txt-2)", deltaLines)
	}

	if len(binaryFiles) != 2 {
		t.Fatalf("expected 2 binary files, got %d: %v", len(binaryFiles), binaryFiles)
	}
	hasImage := false
	hasData := false
	for _, f := range binaryFiles {
		if f == "image.png" {
			hasImage = true
		}
		if f == "data.bin" {
			hasData = true
		}
	}
	if !hasImage {
		t.Errorf("expected image.png in binary files, got %v", binaryFiles)
	}
	if !hasData {
		t.Errorf("expected data.bin in binary files, got %v", binaryFiles)
	}
}

func TestGetBinaryByteDelta(t *testing.T) {
	c := &GitClient{Dir: t.TempDir()}
	setupTestRepo(t, c)

	delta, err := c.GetBinaryByteDelta("main", "feature", []string{"image.png", "data.bin"})
	if err != nil {
		t.Fatalf("GetBinaryByteDelta failed: %v", err)
	}
	if delta != 70 {
		t.Errorf("binary byte delta = %d, want 70 (|100-120| + |0-50|)", delta)
	}
}

func TestGetBinaryByteDelta_NoBinaryFiles(t *testing.T) {
	c := &GitClient{Dir: t.TempDir()}
	setupTestRepo(t, c)

	delta, err := c.GetBinaryByteDelta("main", "feature", nil)
	if err != nil {
		t.Fatalf("GetBinaryByteDelta with nil files failed: %v", err)
	}
	if delta != 0 {
		t.Errorf("empty binary files delta = %d, want 0", delta)
	}
}

func TestGetRepoStats(t *testing.T) {
	c := &GitClient{Dir: t.TempDir()}
	setupTestRepo(t, c)

	loc, binBytes, err := c.GetRepoStats("feature")
	if err != nil {
		t.Fatalf("GetRepoStats failed: %v", err)
	}

	// feature has: a.txt(5) + c.txt(1) = 6 LOC
	if loc != 6 {
		t.Errorf("feature LOC = %d, want 6 (a.txt=5 + c.txt=1)", loc)
	}

	// feature has: image.png(120) + data.bin(50) = 170 bytes
	if binBytes != 170 {
		t.Errorf("feature binary bytes = %d, want 170 (image.png=120 + data.bin=50)", binBytes)
	}
}

func TestGetTextDiff_ReverseBranches(t *testing.T) {
	c := &GitClient{Dir: t.TempDir()}
	setupTestRepo(t, c)

	deltaFwd, _, err := c.GetTextDiff("main", "feature")
	if err != nil {
		t.Fatalf("GetTextDiff(main, feature) failed: %v", err)
	}
	deltaRev, _, err := c.GetTextDiff("feature", "main")
	if err != nil {
		t.Fatalf("GetTextDiff(feature, main) failed: %v", err)
	}
	if deltaFwd != deltaRev {
		t.Errorf("deltaLines should be symmetric: main->feature = %d, feature->main = %d", deltaFwd, deltaRev)
	}
}
