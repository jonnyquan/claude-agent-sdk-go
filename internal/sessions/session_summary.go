// Package sessions: incremental session-summary derivation for SessionStore adapters.
//
// FoldSessionSummary lets a store maintain a per-session SessionSummaryEntry
// sidecar incrementally inside Append() so ListSessionsFromStore() can fetch
// all metadata in a single ListSessionSummaries() call instead of N
// per-session Load() calls.
//
// Every derived field is append-incremental (set-once or last-wins) so
// adapters never need to re-read previously appended entries.
package sessions

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/jonnyquan/claude-agent-sdk-go/internal/shared"
)

// _lastWinsFields maps JSONL entry keys → SessionSummaryEntry data keys for
// last-wins string fields. Each appended entry overwrites the previous value
// when present.
var _lastWinsFields = map[string]string{
	"customTitle": "custom_title",
	"aiTitle":     "ai_title",
	"lastPrompt":  "last_prompt",
	"summary":     "summary_hint",
	"gitBranch":   "git_branch",
}

// FoldSessionSummary folds a batch of appended entries into the running
// summary for key.
//
// Stores call this from inside Append() to keep a SessionSummaryEntry sidecar
// up to date without re-reading the transcript. prev is the previous summary
// for the same key (or nil for the first append).
//
// Do not call this for keys with a Subpath — subagent transcripts must not
// contribute to the main session's summary.
//
// Mtime is NOT touched by the fold — it is the sidecar's storage write time
// and must be stamped by the adapter after persisting.
func FoldSessionSummary(prev *shared.SessionSummaryEntry, key shared.SessionKey, entries []shared.SessionStoreEntry) shared.SessionSummaryEntry {
	var summary shared.SessionSummaryEntry
	if prev != nil {
		summary = shared.SessionSummaryEntry{
			SessionID: prev.SessionID,
			Mtime:     prev.Mtime,
			Data:      copyMap(prev.Data),
		}
	} else {
		summary = shared.SessionSummaryEntry{
			SessionID: key.SessionID,
			Mtime:     0,
			Data:      map[string]any{},
		}
	}
	data := summary.Data

	for _, entry := range entries {
		if entry == nil {
			continue
		}
		ts := isoToEpochMs(entry["timestamp"])

		if _, ok := data["is_sidechain"]; !ok {
			data["is_sidechain"] = entry["isSidechain"] == true
		}
		if _, ok := data["created_at"]; !ok && ts > 0 {
			data["created_at"] = ts
		}

		if _, ok := data["cwd"]; !ok {
			if cwd, ok := entry["cwd"].(string); ok && cwd != "" {
				data["cwd"] = cwd
			}
		}

		foldFirstPrompt(data, entry)

		for src, dst := range _lastWinsFields {
			if val, ok := entry[src].(string); ok {
				data[dst] = val
			}
		}

		if entry["type"] == "tag" {
			if tag, ok := entry["tag"].(string); ok && tag != "" {
				data["tag"] = tag
			} else {
				delete(data, "tag")
			}
		}
	}

	summary.Data = data
	return summary
}

// SummaryEntryToSDKInfo converts a SessionSummaryEntry to SDKSessionInfo.
//
// Returns nil for sidechain sessions or sessions with no extractable summary,
// matching parseSessionInfo's filtering on the disk path.
func SummaryEntryToSDKInfo(entry shared.SessionSummaryEntry, projectPath string) *shared.SDKSessionInfo {
	data := entry.Data
	if isTrue(data["is_sidechain"]) {
		return nil
	}

	var firstPrompt *string
	if isTrue(data["first_prompt_locked"]) {
		if v, ok := data["first_prompt"].(string); ok && v != "" {
			s := v
			firstPrompt = &s
		}
	} else if v, ok := data["command_fallback"].(string); ok && v != "" {
		s := v
		firstPrompt = &s
	}

	var customTitle *string
	if v, ok := data["custom_title"].(string); ok && v != "" {
		s := v
		customTitle = &s
	} else if v, ok := data["ai_title"].(string); ok && v != "" {
		s := v
		customTitle = &s
	}

	var summary string
	if customTitle != nil {
		summary = *customTitle
	}
	if summary == "" {
		if v, ok := data["last_prompt"].(string); ok && v != "" {
			summary = v
		}
	}
	if summary == "" {
		if v, ok := data["summary_hint"].(string); ok && v != "" {
			summary = v
		}
	}
	if summary == "" && firstPrompt != nil {
		summary = *firstPrompt
	}
	if summary == "" {
		return nil
	}

	info := &shared.SDKSessionInfo{
		SessionID:    entry.SessionID,
		Summary:      summary,
		LastModified: entry.Mtime,
		FileSize:     nil,
		CustomTitle:  customTitle,
		FirstPrompt:  firstPrompt,
	}
	if v, ok := data["git_branch"].(string); ok && v != "" {
		s := v
		info.GitBranch = &s
	}
	if v, ok := data["cwd"].(string); ok && v != "" {
		s := v
		info.Cwd = &s
	} else if projectPath != "" {
		s := projectPath
		info.Cwd = &s
	}
	if v, ok := data["tag"].(string); ok && v != "" {
		s := v
		info.Tag = &s
	}
	if ms, ok := toInt64(data["created_at"]); ok {
		info.CreatedAt = &ms
	}
	return info
}

// foldFirstPrompt replicates extractPrompt's first-prompt selection rules over
// a single parsed entry. Mutates data in place: sets first_prompt +
// first_prompt_locked on a real match, or stashes a command_fallback for
// slash-command messages.
func foldFirstPrompt(data map[string]any, entry shared.SessionStoreEntry) {
	if isTrue(data["first_prompt_locked"]) {
		return
	}
	if entry["type"] != "user" {
		return
	}
	if isTrue(entry["isMeta"]) || isTrue(entry["isCompactSummary"]) {
		return
	}

	// Skip tool_result-carrying user messages.
	if message, ok := entry["message"].(map[string]any); ok {
		if content, ok := message["content"].([]any); ok {
			for _, b := range content {
				if block, ok := b.(map[string]any); ok && block["type"] == "tool_result" {
					return
				}
			}
		}
	}

	for _, raw := range entryTextBlocks(entry) {
		result := strings.ReplaceAll(raw, "\n", " ")
		result = strings.TrimSpace(result)
		if result == "" {
			continue
		}
		if m := commandNameRE.FindStringSubmatch(result); m != nil {
			if _, ok := data["command_fallback"]; !ok {
				data["command_fallback"] = m[1]
			}
			continue
		}
		if skipPromptRE.MatchString(result) {
			continue
		}
		if len(result) > 200 {
			result = strings.TrimRight(result[:200], " ") + "…"
		}
		data["first_prompt"] = result
		data["first_prompt_locked"] = true
		return
	}
}

func entryTextBlocks(entry shared.SessionStoreEntry) []string {
	var texts []string
	message, ok := entry["message"].(map[string]any)
	if !ok {
		return texts
	}
	switch content := message["content"].(type) {
	case string:
		if content != "" {
			texts = append(texts, content)
		}
	case []any:
		for _, b := range content {
			block, ok := b.(map[string]any)
			if !ok {
				continue
			}
			if block["type"] == "text" {
				if text, ok := block["text"].(string); ok && text != "" {
					texts = append(texts, text)
				}
			}
		}
	}
	return texts
}

func isoToEpochMs(v any) int64 {
	s, ok := v.(string)
	if !ok || s == "" {
		return 0
	}
	norm := s
	if strings.HasSuffix(norm, "Z") {
		norm = strings.TrimSuffix(norm, "Z") + "+00:00"
	}
	t, err := time.Parse(time.RFC3339Nano, norm)
	if err != nil {
		t, err = time.Parse(time.RFC3339, norm)
		if err != nil {
			return 0
		}
	}
	return t.UnixMilli()
}

func copyMap(m map[string]any) map[string]any {
	cp := make(map[string]any, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return cp
}

func isTrue(v any) bool {
	b, ok := v.(bool)
	return ok && b
}

func toInt64(v any) (int64, bool) {
	switch x := v.(type) {
	case int64:
		return x, true
	case int:
		return int64(x), true
	case float64:
		return int64(x), true
	}
	return 0, false
}

// relPath is a small wrapper that returns "" when paths can't be related.
func relPath(base, target string) (string, error) {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return "", err
	}
	return rel, nil
}

// splitPath splits a relative path into components using either separator,
// so subpaths populated on Windows still resolve correctly when read on POSIX
// (and vice-versa).
func splitPath(p string) []string {
	if p == "" {
		return nil
	}
	parts := []string{}
	last := 0
	for i := 0; i < len(p); i++ {
		if p[i] == '/' || p[i] == filepath.Separator {
			if i > last {
				parts = append(parts, p[last:i])
			}
			last = i + 1
		}
	}
	if last < len(p) {
		parts = append(parts, p[last:])
	}
	return parts
}
