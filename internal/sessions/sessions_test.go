package sessions

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/text/unicode/norm"
)

func TestGetSessionMessagesBuildsMainConversationChain(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", configDir)
	if err := os.Mkdir(filepath.Join(configDir, "projects"), 0o755); err != nil {
		t.Fatalf("mkdir projects: %v", err)
	}

	projectPath := filepath.Join(t.TempDir(), "project")
	if err := os.MkdirAll(projectPath, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}

	projectDir := filepath.Join(configDir, "projects", sanitizePath(canonicalizePath(projectPath)))
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project dir: %v", err)
	}

	sessionID := newUUID()
	u1 := newUUID()
	a1 := newUUID()
	p1 := newUUID()
	sideU := newUUID()

	lines := []string{
		compactJSON(map[string]any{
			"type":      "user",
			"uuid":      u1,
			"sessionId": sessionID,
			"message":   map[string]any{"role": "user", "content": "hello"},
		}),
		compactJSON(map[string]any{
			"type":       "assistant",
			"uuid":       a1,
			"sessionId":  sessionID,
			"parentUuid": u1,
			"message":    map[string]any{"role": "assistant", "content": "hi"},
		}),
		compactJSON(map[string]any{
			"type":       "progress",
			"uuid":       p1,
			"sessionId":  sessionID,
			"parentUuid": a1,
		}),
		compactJSON(map[string]any{
			"type":        "user",
			"uuid":        sideU,
			"sessionId":   sessionID,
			"isSidechain": true,
			"message":     map[string]any{"role": "user", "content": "ignore me"},
		}),
	}

	sessionPath := filepath.Join(projectDir, sessionID+".jsonl")
	if err := os.WriteFile(sessionPath, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}

	messages := GetSessionMessages(sessionID, projectPath, nil, 0)
	if len(messages) != 2 {
		t.Fatalf("expected 2 visible messages, got %d", len(messages))
	}
	if messages[0].UUID != u1 || messages[1].UUID != a1 {
		t.Fatalf("unexpected chain: %#v", messages)
	}
}

func TestTagSessionSanitizesUnicode(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", configDir)
	if err := os.Mkdir(filepath.Join(configDir, "projects"), 0o755); err != nil {
		t.Fatalf("mkdir projects: %v", err)
	}

	projectPath := filepath.Join(t.TempDir(), "project")
	if err := os.MkdirAll(projectPath, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}

	projectDir := filepath.Join(configDir, "projects", sanitizePath(canonicalizePath(projectPath)))
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project dir: %v", err)
	}

	sessionID := newUUID()
	sessionPath := filepath.Join(projectDir, sessionID+".jsonl")
	lines := []string{
		compactJSON(map[string]any{
			"type":      "user",
			"uuid":      newUUID(),
			"sessionId": sessionID,
			"message":   map[string]any{"role": "user", "content": "hello"},
		}),
	}
	if err := os.WriteFile(sessionPath, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}

	tag := "a\u200bb"
	if err := TagSession(sessionID, &tag, projectPath); err != nil {
		t.Fatalf("tag session: %v", err)
	}

	data, err := os.ReadFile(sessionPath)
	if err != nil {
		t.Fatalf("read session: %v", err)
	}
	if !strings.Contains(string(data), `"tag":"ab"`) {
		t.Fatalf("expected sanitized tag entry, got %s", string(data))
	}
}

func TestForkSessionSkipsProgressAndPreservesVisibleConversation(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", configDir)
	if err := os.Mkdir(filepath.Join(configDir, "projects"), 0o755); err != nil {
		t.Fatalf("mkdir projects: %v", err)
	}

	projectPath := filepath.Join(t.TempDir(), "project")
	if err := os.MkdirAll(projectPath, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}

	projectDir := filepath.Join(configDir, "projects", sanitizePath(canonicalizePath(projectPath)))
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project dir: %v", err)
	}

	sessionID := newUUID()
	u1 := newUUID()
	a1 := newUUID()
	p1 := newUUID()
	u2 := newUUID()
	lines := []string{
		compactJSON(map[string]any{
			"type":      "user",
			"uuid":      u1,
			"sessionId": sessionID,
			"message":   map[string]any{"role": "user", "content": "hello"},
		}),
		compactJSON(map[string]any{
			"type":       "assistant",
			"uuid":       a1,
			"sessionId":  sessionID,
			"parentUuid": u1,
			"message":    map[string]any{"role": "assistant", "content": "hi"},
		}),
		compactJSON(map[string]any{
			"type":       "progress",
			"uuid":       p1,
			"sessionId":  sessionID,
			"parentUuid": a1,
		}),
		compactJSON(map[string]any{
			"type":       "user",
			"uuid":       u2,
			"sessionId":  sessionID,
			"parentUuid": p1,
			"message":    map[string]any{"role": "user", "content": "follow up"},
		}),
	}
	sessionPath := filepath.Join(projectDir, sessionID+".jsonl")
	if err := os.WriteFile(sessionPath, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}

	result, err := ForkSession(sessionID, projectPath, nil, nil)
	if err != nil {
		t.Fatalf("fork session: %v", err)
	}

	messages := GetSessionMessages(result.SessionID, projectPath, nil, 0)
	if len(messages) != 3 {
		t.Fatalf("expected 3 visible messages in fork, got %d", len(messages))
	}
	if messages[0].UUID == u1 || messages[1].UUID == a1 || messages[2].UUID == u2 {
		t.Fatalf("expected remapped UUIDs in fork, got %#v", messages)
	}

	forkedPath := filepath.Join(projectDir, result.SessionID+".jsonl")
	data, err := os.ReadFile(forkedPath)
	if err != nil {
		t.Fatalf("read fork: %v", err)
	}
	if strings.Contains(string(data), `"type":"progress"`) {
		t.Fatalf("fork should not persist progress entries: %s", string(data))
	}

	var entries []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("decode fork line: %v", err)
		}
		entries = append(entries, entry)
	}

	foundForkedFrom := false
	for _, entry := range entries {
		if _, ok := entry["forkedFrom"]; ok {
			foundForkedFrom = true
			break
		}
	}
	if !foundForkedFrom {
		t.Fatal("expected forked transcript entries to include forkedFrom metadata")
	}
}

func TestListSessionsKeepsNewestDuplicateSession(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", configDir)
	projectsDir := filepath.Join(configDir, "projects")
	if err := os.MkdirAll(projectsDir, 0o755); err != nil {
		t.Fatalf("mkdir projects: %v", err)
	}

	sessionID := newUUID()
	oldProjectPath := filepath.Join(t.TempDir(), "project-old")
	newProjectPath := filepath.Join(t.TempDir(), "project-new")
	if err := os.MkdirAll(oldProjectPath, 0o755); err != nil {
		t.Fatalf("mkdir old project: %v", err)
	}
	if err := os.MkdirAll(newProjectPath, 0o755); err != nil {
		t.Fatalf("mkdir new project: %v", err)
	}

	oldProjectDir := filepath.Join(projectsDir, sanitizePath(canonicalizePath(oldProjectPath)))
	newProjectDir := filepath.Join(projectsDir, sanitizePath(canonicalizePath(newProjectPath)))
	if err := os.MkdirAll(oldProjectDir, 0o755); err != nil {
		t.Fatalf("mkdir old project dir: %v", err)
	}
	if err := os.MkdirAll(newProjectDir, 0o755); err != nil {
		t.Fatalf("mkdir new project dir: %v", err)
	}

	oldPath := filepath.Join(oldProjectDir, sessionID+".jsonl")
	newPath := filepath.Join(newProjectDir, sessionID+".jsonl")
	oldContent := compactJSON(map[string]any{
		"type":      "user",
		"uuid":      newUUID(),
		"sessionId": sessionID,
		"message":   map[string]any{"role": "user", "content": "old summary"},
	}) + "\n"
	newContent := compactJSON(map[string]any{
		"type":      "user",
		"uuid":      newUUID(),
		"sessionId": sessionID,
		"message":   map[string]any{"role": "user", "content": "new summary"},
	}) + "\n"
	if err := os.WriteFile(oldPath, []byte(oldContent), 0o644); err != nil {
		t.Fatalf("write old session: %v", err)
	}
	if err := os.WriteFile(newPath, []byte(newContent), 0o644); err != nil {
		t.Fatalf("write new session: %v", err)
	}

	oldTime := time.UnixMilli(1_700_000_000_000)
	newTime := oldTime.Add(2 * time.Minute)
	if err := os.Chtimes(oldPath, oldTime, oldTime); err != nil {
		t.Fatalf("chtimes old session: %v", err)
	}
	if err := os.Chtimes(newPath, newTime, newTime); err != nil {
		t.Fatalf("chtimes new session: %v", err)
	}

	sessions := ListSessions("", nil, 0, true)
	if len(sessions) != 1 {
		t.Fatalf("expected 1 deduplicated session, got %d", len(sessions))
	}
	if sessions[0].Summary != "new summary" {
		t.Fatalf("expected newest duplicate to win, got summary %q", sessions[0].Summary)
	}
	if sessions[0].LastModified != newTime.UnixMilli() {
		t.Fatalf("expected newest last_modified %d, got %d", newTime.UnixMilli(), sessions[0].LastModified)
	}
}

func TestGetSessionInfoUsesLastPromptAndAITitleFallbacks(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", configDir)
	if err := os.Mkdir(filepath.Join(configDir, "projects"), 0o755); err != nil {
		t.Fatalf("mkdir projects: %v", err)
	}

	projectPath := filepath.Join(t.TempDir(), "project")
	if err := os.MkdirAll(projectPath, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}

	projectDir := filepath.Join(configDir, "projects", sanitizePath(canonicalizePath(projectPath)))
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project dir: %v", err)
	}

	sessionID := newUUID()
	sessionPath := filepath.Join(projectDir, sessionID+".jsonl")
	lines := []string{
		compactJSON(map[string]any{
			"type":      "user",
			"uuid":      newUUID(),
			"sessionId": sessionID,
			"message":   map[string]any{"role": "user", "content": "first prompt"},
		}),
		compactJSON(map[string]any{
			"type":       "summary",
			"sessionId":  sessionID,
			"aiTitle":    "AI title",
			"lastPrompt": "recent prompt",
		}),
	}
	if err := os.WriteFile(sessionPath, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}

	info := GetSessionInfo(sessionID, projectPath)
	if info == nil {
		t.Fatal("expected session info")
	}
	if info.Summary != "AI title" {
		t.Fatalf("expected aiTitle to win summary fallback, got %q", info.Summary)
	}
	if info.CustomTitle == nil || *info.CustomTitle != "AI title" {
		t.Fatalf("expected custom_title fallback to aiTitle, got %#v", info.CustomTitle)
	}

	lines = append(lines, compactJSON(map[string]any{
		"type":       "summary",
		"sessionId":  sessionID,
		"lastPrompt": "latest prompt",
	}))
	if err := os.WriteFile(sessionPath, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("rewrite session: %v", err)
	}

	info = GetSessionInfo(sessionID, projectPath)
	if info == nil {
		t.Fatal("expected session info after rewrite")
	}
	if info.Summary != "AI title" {
		t.Fatalf("expected aiTitle to remain preferred over lastPrompt, got %q", info.Summary)
	}
}

func TestGetSessionInfoUsesLastPromptWhenNoTitleExists(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", configDir)
	if err := os.Mkdir(filepath.Join(configDir, "projects"), 0o755); err != nil {
		t.Fatalf("mkdir projects: %v", err)
	}

	projectPath := filepath.Join(t.TempDir(), "project")
	if err := os.MkdirAll(projectPath, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}

	projectDir := filepath.Join(configDir, "projects", sanitizePath(canonicalizePath(projectPath)))
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project dir: %v", err)
	}

	sessionID := newUUID()
	sessionPath := filepath.Join(projectDir, sessionID+".jsonl")
	lines := []string{
		compactJSON(map[string]any{
			"type":      "user",
			"uuid":      newUUID(),
			"sessionId": sessionID,
			"message":   map[string]any{"role": "user", "content": "first prompt"},
		}),
		compactJSON(map[string]any{
			"type":       "summary",
			"sessionId":  sessionID,
			"lastPrompt": "latest prompt",
			"summary":    "older summary",
		}),
	}
	if err := os.WriteFile(sessionPath, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}

	info := GetSessionInfo(sessionID, projectPath)
	if info == nil {
		t.Fatal("expected session info")
	}
	if info.Summary != "latest prompt" {
		t.Fatalf("expected lastPrompt to win over summary, got %q", info.Summary)
	}
}

func TestForkSessionDerivesTitleFromAITitleBeforeFirstPrompt(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", configDir)
	if err := os.Mkdir(filepath.Join(configDir, "projects"), 0o755); err != nil {
		t.Fatalf("mkdir projects: %v", err)
	}

	projectPath := filepath.Join(t.TempDir(), "project")
	if err := os.MkdirAll(projectPath, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}

	projectDir := filepath.Join(configDir, "projects", sanitizePath(canonicalizePath(projectPath)))
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project dir: %v", err)
	}

	sessionID := newUUID()
	lines := []string{
		compactJSON(map[string]any{
			"type":      "user",
			"uuid":      newUUID(),
			"sessionId": sessionID,
			"message":   map[string]any{"role": "user", "content": "first prompt"},
		}),
		compactJSON(map[string]any{
			"type":      "summary",
			"sessionId": sessionID,
			"aiTitle":   "AI title",
		}),
	}
	sessionPath := filepath.Join(projectDir, sessionID+".jsonl")
	if err := os.WriteFile(sessionPath, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}

	result, err := ForkSession(sessionID, projectPath, nil, nil)
	if err != nil {
		t.Fatalf("fork session: %v", err)
	}

	forkedPath := filepath.Join(projectDir, result.SessionID+".jsonl")
	data, err := os.ReadFile(forkedPath)
	if err != nil {
		t.Fatalf("read fork: %v", err)
	}
	if !strings.Contains(string(data), `"customTitle":"AI title (fork)"`) {
		t.Fatalf("expected aiTitle-derived fork title, got %s", string(data))
	}
}

func TestGetSessionInfoSkipsIDEInjectedPromptBlocks(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", configDir)
	if err := os.Mkdir(filepath.Join(configDir, "projects"), 0o755); err != nil {
		t.Fatalf("mkdir projects: %v", err)
	}

	projectPath := filepath.Join(t.TempDir(), "project")
	if err := os.MkdirAll(projectPath, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}

	projectDir := filepath.Join(configDir, "projects", sanitizePath(canonicalizePath(projectPath)))
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project dir: %v", err)
	}

	sessionID := newUUID()
	sessionPath := filepath.Join(projectDir, sessionID+".jsonl")
	lines := []string{
		compactJSON(map[string]any{
			"type":      "user",
			"uuid":      newUUID(),
			"sessionId": sessionID,
			"message": map[string]any{
				"role":    "user",
				"content": "<ide_selection>ignored</ide_selection>",
			},
		}),
		compactJSON(map[string]any{
			"type":      "user",
			"uuid":      newUUID(),
			"sessionId": sessionID,
			"message": map[string]any{
				"role":    "user",
				"content": "real prompt",
			},
		}),
	}
	if err := os.WriteFile(sessionPath, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}

	info := GetSessionInfo(sessionID, projectPath)
	if info == nil {
		t.Fatal("expected session info")
	}
	if info.Summary != "real prompt" {
		t.Fatalf("expected IDE prompt blocks to be skipped, got %q", info.Summary)
	}
}

func TestCanonicalizePathNormalizesUnicodeToNFC(t *testing.T) {
	decomposed := filepath.Join(t.TempDir(), "Cafe\u0301")
	got := canonicalizePath(decomposed)
	want, err := filepath.Abs(filepath.Join(filepath.Dir(decomposed), "Caf\u00e9"))
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}
	want = norm.NFC.String(want)
	if got != want {
		t.Fatalf("expected NFC-normalized path %q, got %q", want, got)
	}
}

func TestGetClaudeConfigHomeDirNormalizesUnicodeToNFC(t *testing.T) {
	configDir := filepath.Join(t.TempDir(), "Claud\u0065\u0301")
	t.Setenv("CLAUDE_CONFIG_DIR", configDir)

	got := getClaudeConfigHomeDir()
	want := norm.NFC.String(configDir)
	if got != want {
		t.Fatalf("expected normalized config dir %q, got %q", want, got)
	}
}
