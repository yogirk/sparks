package core

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sahilm/fuzzy"

	"github.com/yogirk/sparks/internal/vault"
)

// TasksFilename is the relative path to the live Tasks collection page.
// This page is the exception to the "collections are regenerated" rule —
// it's edited in place, owned by the agent and by `sparks tasks/done`.
const TasksFilename = "wiki/collections/Tasks.md"

// TaskAddResult reports what happened during a tasks add.
type TaskAddResult struct {
	Section       string `json:"section"`
	SectionCreate bool   `json:"section_created"`
	Text          string `json:"text"`
	Line          int    `json:"line"`
}

// TaskDoneResult reports what happened during a task done.
type TaskDoneResult struct {
	Matched    string   `json:"matched"`
	Line       int      `json:"line"`
	Ambiguous  bool     `json:"ambiguous,omitempty"`
	Candidates []string `json:"candidates,omitempty"`
}

// Sentinel errors for task operations.
var (
	ErrTaskAmbiguous = errors.New("tasks: multiple matches, refine the query")
	ErrTaskNotFound  = errors.New("tasks: no matching open task")
)

// TasksAdd appends a new `- [ ] text` line under the given section heading
// in wiki/collections/Tasks.md. The section heading is matched as `## {section}`
// or `### {section}`. If the section is missing, it's created at the end
// of the file with a `## ` prefix.
//
// section is typically a wikilink like `[[Sparks]]`. We don't parse or
// normalize it — pass whatever the agent decided. Duplicate-suppression
// is the agent's concern: TasksAdd always appends.
func TasksAdd(v *vault.Vault, section, text string) (TaskAddResult, error) {
	section = strings.TrimSpace(section)
	text = strings.TrimSpace(text)
	if section == "" || text == "" {
		return TaskAddResult{}, fmt.Errorf("tasks add: section and text required")
	}
	path := filepath.Join(v.Root, TasksFilename)
	lines, created, err := ensureTasksFile(path)
	if err != nil {
		return TaskAddResult{}, err
	}
	sectionCreated := false
	sectionIdx := findSection(lines, section)
	if sectionIdx < 0 {
		// Append section header at end of file.
		if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
			lines = append(lines, "")
		}
		lines = append(lines, "## "+section, "")
		sectionIdx = len(lines) - 1
		sectionCreated = true
	}

	// Insert the task right after the last task line (or heading) within
	// this section — keeps ordering intuitive as tasks accumulate.
	insertAt := tailOfSection(lines, sectionIdx)
	taskLine := "- [ ] " + text
	lines = insert(lines, insertAt, taskLine)

	if err := writeLines(path, lines); err != nil {
		return TaskAddResult{}, err
	}
	return TaskAddResult{
		Section:       section,
		SectionCreate: sectionCreated || created,
		Text:          text,
		Line:          insertAt + 1, // 1-based for humans
	}, nil
}

// TaskDone marks an open task complete. Exact match wins over fuzzy. If
// multiple fuzzy candidates remain, returns ErrTaskAmbiguous with the list.
// If none match, returns ErrTaskNotFound.
func TaskDone(v *vault.Vault, query string) (TaskDoneResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return TaskDoneResult{}, fmt.Errorf("tasks done: query required")
	}
	path := filepath.Join(v.Root, TasksFilename)
	lines, _, err := ensureTasksFile(path)
	if err != nil {
		return TaskDoneResult{}, err
	}

	var open []taskCandidate
	for i, l := range lines {
		t, ok := openTaskText(l)
		if !ok {
			continue
		}
		open = append(open, taskCandidate{line: i, text: t})
	}
	if len(open) == 0 {
		return TaskDoneResult{}, ErrTaskNotFound
	}

	// Exact substring match wins outright.
	var exactMatches []taskCandidate
	lowerQ := strings.ToLower(query)
	for _, c := range open {
		if strings.Contains(strings.ToLower(c.text), lowerQ) {
			exactMatches = append(exactMatches, c)
		}
	}
	if len(exactMatches) == 1 {
		return completeTask(path, lines, exactMatches[0].line, exactMatches[0].text)
	}
	if len(exactMatches) > 1 {
		return TaskDoneResult{
			Ambiguous:  true,
			Candidates: candidateStrings(exactMatches),
		}, ErrTaskAmbiguous
	}

	// Fall back to fuzzy.
	texts := make([]string, len(open))
	for i, c := range open {
		texts[i] = c.text
	}
	matches := fuzzy.Find(query, texts)
	if len(matches) == 0 {
		return TaskDoneResult{}, ErrTaskNotFound
	}
	if len(matches) == 1 || (len(matches) > 1 && matches[0].Score > matches[1].Score+5) {
		m := matches[0]
		chosen := open[m.Index]
		return completeTask(path, lines, chosen.line, chosen.text)
	}
	// Multiple close matches — ambiguous.
	var cands []taskCandidate
	for i := 0; i < len(matches) && i < 5; i++ {
		cands = append(cands, open[matches[i].Index])
	}
	return TaskDoneResult{
		Ambiguous:  true,
		Candidates: candidateStrings(cands),
	}, ErrTaskAmbiguous
}

func completeTask(path string, lines []string, idx int, text string) (TaskDoneResult, error) {
	lines[idx] = strings.Replace(lines[idx], "- [ ]", "- [x]", 1)
	if err := writeLines(path, lines); err != nil {
		return TaskDoneResult{}, err
	}
	return TaskDoneResult{Matched: text, Line: idx + 1}, nil
}

// ensureTasksFile reads the Tasks.md file, creating an empty one with a
// seed heading if missing. Returns lines (without trailing newlines) and a
// bool indicating whether the file was created this call.
func ensureTasksFile(path string) ([]string, bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		seed := "# Tasks\n\nLive task list. `- [ ]` tracked, `- [x]` completed. Managed by Sparks.\n"
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, false, err
		}
		if err := os.WriteFile(path, []byte(seed), 0o644); err != nil {
			return nil, false, err
		}
		return splitLines(seed), true, nil
	}
	if err != nil {
		return nil, false, err
	}
	return splitLines(string(data)), false, nil
}

// findSection returns the line index of a `##` or `###` heading matching
// name, or -1 if absent. Matching is exact after trimming trailing
// whitespace; agents pass pre-normalized strings.
func findSection(lines []string, name string) int {
	want := []string{"## " + name, "### " + name}
	for i, l := range lines {
		for _, w := range want {
			if strings.TrimRight(l, " \t") == w {
				return i
			}
		}
	}
	return -1
}

// tailOfSection returns the index at which to insert a new task within the
// section whose heading is at lines[sectionIdx]. The insertion point is
// right after the last non-blank line that belongs to the section (before
// the next heading or end of file).
func tailOfSection(lines []string, sectionIdx int) int {
	last := sectionIdx
	for i := sectionIdx + 1; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "## ") || strings.HasPrefix(lines[i], "### ") {
			break
		}
		if strings.TrimSpace(lines[i]) != "" {
			last = i
		}
	}
	return last + 1
}

// openTaskText extracts the text portion of an unchecked task line,
// returning ("", false) for any line that isn't an open task.
func openTaskText(line string) (string, bool) {
	trimmed := strings.TrimLeft(line, " \t")
	const prefix = "- [ ] "
	if !strings.HasPrefix(trimmed, prefix) {
		return "", false
	}
	return strings.TrimSpace(trimmed[len(prefix):]), true
}

// taskCandidate pairs a matched open-task line with its 0-based index.
// Kept package-scoped so it's shareable between matchers and formatters.
type taskCandidate struct {
	line int
	text string
}

func candidateStrings(cs []taskCandidate) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.text
	}
	return out
}

func splitLines(s string) []string {
	scanner := bufio.NewScanner(strings.NewReader(s))
	scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines
}

func insert(lines []string, at int, value string) []string {
	if at >= len(lines) {
		return append(lines, value)
	}
	out := make([]string, 0, len(lines)+1)
	out = append(out, lines[:at]...)
	out = append(out, value)
	out = append(out, lines[at:]...)
	return out
}

func writeLines(path string, lines []string) error {
	body := strings.Join(lines, "\n")
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	return os.WriteFile(path, []byte(body), 0o644)
}
