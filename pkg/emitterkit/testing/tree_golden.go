package testing

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/galihanggara68/foundation/pkg/emitterkit"
)

// TreeGoldenOptions configures the behavior of tree golden comparisons.
type TreeGoldenOptions struct {
	// UpdateGoldens overwrites golden files on disk with response contents when true.
	UpdateGoldens bool
	// IgnorePaths lists relative path patterns to skip during comparison.
	IgnorePaths []string
}

// AssertGoldenTree compares emitted files in resp against golden files on disk.
func (h *Harness) AssertGoldenTree(resp emitterkit.GenerationResponse, goldenDir string, opts TreeGoldenOptions) {
	h.t.Helper()

	respFiles := make(map[string]string)
	for relPath, content := range resp.Stubs {
		respFiles[filepath.ToSlash(relPath)] = content
	}
	for relPath, content := range resp.Files {
		respFiles[filepath.ToSlash(relPath)] = content
	}

	for relPath := range respFiles {
		if isIgnoredPath(relPath, opts.IgnorePaths) {
			delete(respFiles, relPath)
		}
	}

	if opts.UpdateGoldens {
		h.updateGoldenFiles(goldenDir, respFiles, opts.IgnorePaths)
		return
	}

	diskFiles := h.collectDiskFiles(goldenDir, opts.IgnorePaths)
	if diskFiles == nil {
		return
	}
	h.compareFileSets(respFiles, diskFiles)
	h.compareFileContents(goldenDir, respFiles, diskFiles)
}

func (h *Harness) updateGoldenFiles(goldenDir string, respFiles map[string]string, ignorePaths []string) {
	h.t.Helper()

	if _, err := os.Stat(goldenDir); err == nil {
		err := filepath.Walk(goldenDir, func(path string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if info.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(goldenDir, path)
			if err != nil {
				return err
			}
			relSlash := filepath.ToSlash(rel)
			if isIgnoredPath(relSlash, ignorePaths) {
				return nil
			}
			if _, ok := respFiles[relSlash]; !ok {
				if err := os.Remove(path); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			h.t.Fatalf("clean old golden files: %v", err)
			return
		}
	}

	for relSlash, content := range respFiles {
		destPath := filepath.Join(goldenDir, filepath.FromSlash(relSlash))
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			h.t.Fatalf("create directory for golden file %q: %v", relSlash, err)
			return
		}
		if err := os.WriteFile(destPath, []byte(content), 0644); err != nil {
			h.t.Fatalf("write golden file %q: %v", relSlash, err)
			return
		}
	}
}

func (h *Harness) collectDiskFiles(goldenDir string, ignorePaths []string) map[string]string {
	h.t.Helper()

	if _, err := os.Stat(goldenDir); os.IsNotExist(err) {
		h.t.Fatalf("golden directory %s does not exist", goldenDir)
		return nil
	}

	diskFiles := make(map[string]string)
	err := filepath.Walk(goldenDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(goldenDir, path)
		if err != nil {
			return err
		}
		relSlash := filepath.ToSlash(rel)
		if isIgnoredPath(relSlash, ignorePaths) {
			return nil
		}
		diskFiles[relSlash] = path
		return nil
	})
	if err != nil {
		h.t.Fatalf("read golden directory %s: %v", goldenDir, err)
		return nil
	}
	return diskFiles
}

func (h *Harness) compareFileSets(respFiles map[string]string, diskFiles map[string]string) {
	h.t.Helper()

	var missingInResp []string
	for rel := range diskFiles {
		if _, ok := respFiles[rel]; !ok {
			missingInResp = append(missingInResp, rel)
		}
	}
	sort.Strings(missingInResp)
	for _, rel := range missingInResp {
		h.t.Errorf("missing expected golden file %q in response", rel)
	}

	var extraInResp []string
	for rel := range respFiles {
		if _, ok := diskFiles[rel]; !ok {
			extraInResp = append(extraInResp, rel)
		}
	}
	sort.Strings(extraInResp)
	for _, rel := range extraInResp {
		h.t.Errorf("unexpected emitted file %q in response", rel)
	}
}

func (h *Harness) compareFileContents(goldenDir string, respFiles, diskFiles map[string]string) {
	h.t.Helper()

	var commonPaths []string
	for rel := range respFiles {
		if _, ok := diskFiles[rel]; ok {
			commonPaths = append(commonPaths, rel)
		}
	}
	sort.Strings(commonPaths)

	for _, rel := range commonPaths {
		diskPath := diskFiles[rel]
		diskBytes, err := os.ReadFile(diskPath)
		if err != nil {
			h.t.Errorf("read golden file %q: %v", rel, err)
			continue
		}
		diskContent := string(diskBytes)
		respContent := respFiles[rel]
		if diskContent != respContent {
			diff := formatUnifiedDiff(rel, diskContent, respContent)
			h.t.Errorf("file %q content mismatch:\n%s", rel, diff)
		}
	}
}

func isIgnoredPath(relPath string, ignorePaths []string) bool {
	for _, pattern := range ignorePaths {
		if pattern == relPath {
			return true
		}
		if matched, _ := filepath.Match(pattern, relPath); matched {
			return true
		}
		if matched, _ := filepath.Match(pattern, filepath.Base(relPath)); matched {
			return true
		}
	}
	return false
}

func formatUnifiedDiff(name, expected, actual string) string {
	expLines := splitDiffLines(expected)
	actLines := splitDiffLines(actual)

	ops := computeDiffOperations(expLines, actLines)
	if len(ops) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("--- golden/%s\n", name))
	sb.WriteString(fmt.Sprintf("+++ emitted/%s\n", name))

	hunks := buildDiffHunks(ops, 3)
	for _, hunk := range hunks {
		sb.WriteString(fmt.Sprintf("@@ -%d,%d +%d,%d @@\n", hunk.oldStart, hunk.oldLen, hunk.newStart, hunk.newLen))
		for _, line := range hunk.lines {
			sb.WriteString(line)
			sb.WriteByte('\n')
		}
	}

	return sb.String()
}

func splitDiffLines(s string) []string {
	normalized := strings.ReplaceAll(s, "\r\n", "\n")
	if len(normalized) == 0 {
		return nil
	}
	lines := strings.Split(normalized, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

type diffOperation struct {
	kind rune
	text string
}

func computeDiffOperations(a, b []string) []diffOperation {
	n := len(a)
	m := len(b)
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else {
				if dp[i+1][j] >= dp[i][j+1] {
					dp[i][j] = dp[i+1][j]
				} else {
					dp[i][j] = dp[i][j+1]
				}
			}
		}
	}

	var ops []diffOperation
	i, j := 0, 0
	for i < n && j < m {
		if a[i] == b[j] {
			ops = append(ops, diffOperation{kind: ' ', text: a[i]})
			i++
			j++
		} else if dp[i+1][j] >= dp[i][j+1] {
			ops = append(ops, diffOperation{kind: '-', text: a[i]})
			i++
		} else {
			ops = append(ops, diffOperation{kind: '+', text: b[j]})
			j++
		}
	}
	for ; i < n; i++ {
		ops = append(ops, diffOperation{kind: '-', text: a[i]})
	}
	for ; j < m; j++ {
		ops = append(ops, diffOperation{kind: '+', text: b[j]})
	}
	return ops
}

type diffHunk struct {
	oldStart int
	oldLen   int
	newStart int
	newLen   int
	lines    []string
}

func buildDiffHunks(ops []diffOperation, contextLines int) []diffHunk {
	var hasChange bool
	for _, op := range ops {
		if op.kind != ' ' {
			hasChange = true
			break
		}
	}
	if !hasChange {
		return nil
	}

	type annotatedOp struct {
		op      diffOperation
		oldLine int
		newLine int
	}

	annotated := make([]annotatedOp, len(ops))
	curOld := 1
	curNew := 1
	for idx, op := range ops {
		annotated[idx] = annotatedOp{
			op:      op,
			oldLine: curOld,
			newLine: curNew,
		}
		switch op.kind {
		case ' ':
			curOld++
			curNew++
		case '-':
			curOld++
		case '+':
			curNew++
		}
	}

	var changeIndices []int
	for idx, ann := range annotated {
		if ann.op.kind != ' ' {
			changeIndices = append(changeIndices, idx)
		}
	}

	type hunkRange struct {
		startIdx int
		endIdx   int
	}

	var ranges []hunkRange
	for _, cIdx := range changeIndices {
		start := cIdx - contextLines
		if start < 0 {
			start = 0
		}
		end := cIdx + contextLines
		if end >= len(annotated) {
			end = len(annotated) - 1
		}
		if len(ranges) == 0 {
			ranges = append(ranges, hunkRange{startIdx: start, endIdx: end})
			continue
		}
		last := &ranges[len(ranges)-1]
		if start <= last.endIdx+1 {
			if end > last.endIdx {
				last.endIdx = end
			}
		} else {
			ranges = append(ranges, hunkRange{startIdx: start, endIdx: end})
		}
	}

	var hunks []diffHunk
	for _, r := range ranges {
		hunkOps := annotated[r.startIdx : r.endIdx+1]
		if len(hunkOps) == 0 {
			continue
		}
		first := hunkOps[0]
		hunk := diffHunk{
			oldStart: first.oldLine,
			newStart: first.newLine,
		}
		for _, ann := range hunkOps {
			hunk.lines = append(hunk.lines, fmt.Sprintf("%c%s", ann.op.kind, ann.op.text))
			switch ann.op.kind {
			case ' ':
				hunk.oldLen++
				hunk.newLen++
			case '-':
				hunk.oldLen++
			case '+':
				hunk.newLen++
			}
		}
		if hunk.oldLen == 0 {
			hunk.oldStart = 0
		}
		if hunk.newLen == 0 {
			hunk.newStart = 0
		}
		hunks = append(hunks, hunk)
	}

	return hunks
}
