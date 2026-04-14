package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTasksAddCreatesSectionAndFile(t *testing.T) {
	v, db := freshVault(t)
	_ = db

	res, err := TasksAdd(v, "[[Sparks]]", "Write ingest finalize tests")
	if err != nil {
		t.Fatalf("TasksAdd: %v", err)
	}
	if !res.SectionCreate {
		t.Error("SectionCreate = false, want true on first add")
	}

	body := readVaultFile(t, v, TasksFilename)
	if !strings.Contains(body, "## [[Sparks]]") {
		t.Errorf("section not written: %q", body)
	}
	if !strings.Contains(body, "- [ ] Write ingest finalize tests") {
		t.Errorf("task not written: %q", body)
	}
}

func TestTasksAddAppendsUnderExistingSection(t *testing.T) {
	v, _ := freshVault(t)
	if _, err := TasksAdd(v, "[[Sparks]]", "first"); err != nil {
		t.Fatal(err)
	}
	res2, err := TasksAdd(v, "[[Sparks]]", "second")
	if err != nil {
		t.Fatal(err)
	}
	if res2.SectionCreate {
		t.Error("SectionCreate = true on second add, want false")
	}
	body := readVaultFile(t, v, TasksFilename)
	if !strings.Contains(body, "first") || !strings.Contains(body, "second") {
		t.Errorf("both tasks should be present:\n%s", body)
	}
	if strings.Count(body, "## [[Sparks]]") != 1 {
		t.Errorf("section duplicated:\n%s", body)
	}
}

func TestTasksAddRejectsEmptyInputs(t *testing.T) {
	v, _ := freshVault(t)
	if _, err := TasksAdd(v, "", "text"); err == nil {
		t.Error("empty section should error")
	}
	if _, err := TasksAdd(v, "[[Sparks]]", ""); err == nil {
		t.Error("empty text should error")
	}
}

func TestTaskDoneExactMatchToggles(t *testing.T) {
	v, _ := freshVault(t)
	if _, err := TasksAdd(v, "[[Sparks]]", "ship ingest prepare"); err != nil {
		t.Fatal(err)
	}
	res, err := TaskDone(v, "ship ingest prepare")
	if err != nil {
		t.Fatalf("TaskDone: %v", err)
	}
	if res.Matched != "ship ingest prepare" {
		t.Errorf("Matched = %q", res.Matched)
	}
	body := readVaultFile(t, v, TasksFilename)
	if !strings.Contains(body, "- [x] ship ingest prepare") {
		t.Errorf("task not toggled:\n%s", body)
	}
}

func TestTaskDoneFuzzyMatch(t *testing.T) {
	v, _ := freshVault(t)
	if _, err := TasksAdd(v, "[[Sparks]]", "implement the ingest finalize path"); err != nil {
		t.Fatal(err)
	}
	res, err := TaskDone(v, "finalize")
	if err != nil {
		t.Fatalf("TaskDone fuzzy: %v", err)
	}
	if !strings.Contains(res.Matched, "finalize") {
		t.Errorf("Matched = %q", res.Matched)
	}
}

func TestTaskDoneAmbiguousReturnsCandidates(t *testing.T) {
	v, _ := freshVault(t)
	_, _ = TasksAdd(v, "[[Sparks]]", "write tests for ingest")
	_, _ = TasksAdd(v, "[[Sparks]]", "write tests for scan")
	res, err := TaskDone(v, "write tests")
	if !errors.Is(err, ErrTaskAmbiguous) {
		t.Fatalf("err = %v, want ErrTaskAmbiguous", err)
	}
	if len(res.Candidates) < 2 {
		t.Errorf("Candidates = %v, want at least 2", res.Candidates)
	}
}

func TestTaskDoneNotFound(t *testing.T) {
	v, _ := freshVault(t)
	_, _ = TasksAdd(v, "[[Sparks]]", "something unrelated")
	_, err := TaskDone(v, "totally absent phrase")
	if !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("err = %v, want ErrTaskNotFound", err)
	}
}

func TestTaskDoneAlreadyCompletedIsNotMatched(t *testing.T) {
	v, _ := freshVault(t)
	_, _ = TasksAdd(v, "[[Sparks]]", "completed task")
	if _, err := TaskDone(v, "completed task"); err != nil {
		t.Fatal(err)
	}
	// Second attempt should not match — it's already [x].
	_, err := TaskDone(v, "completed task")
	if !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("re-done err = %v, want ErrTaskNotFound", err)
	}
}

func readVaultFile(t *testing.T, v interface{ ManifestPath() string }, rel string) string {
	t.Helper()
	// v isn't typed here to avoid an import loop; use its ManifestPath to
	// derive the vault root instead of importing vault directly.
	root := filepath.Dir(v.ManifestPath())
	data, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}
