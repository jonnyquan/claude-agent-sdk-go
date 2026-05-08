package sessions

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/jonnyquan/claude-agent-sdk-go/internal/shared"
	"golang.org/x/text/unicode/norm"
)

const maxSanitizedLength = 200

var (
	uuidRE          = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	sanitizeRE      = regexp.MustCompile(`[^a-zA-Z0-9]`)
	commandNameRE   = regexp.MustCompile(`<command-name>(.*?)</command-name>`)
	skipPromptRE    = regexp.MustCompile(`^(?:<local-command-stdout>|<session-start-hook>|<tick>|<goal>|\[Request interrupted by user[^\]]*\]|\s*<ide_opened_file>[\s\S]*</ide_opened_file>\s*$|\s*<ide_selection>[\s\S]*</ide_selection>\s*$)`)
	transcriptTypes = map[string]bool{
		shared.MessageTypeUser:      true,
		shared.MessageTypeAssistant: true,
		"progress":                  true,
		shared.MessageTypeSystem:    true,
		"attachment":                true,
	}
)

type transcriptEntry struct {
	Type            string
	UUID            string
	SessionID       string
	Message         map[string]any
	ParentToolUseID *string
	ParentUUID      *string
	Raw             map[string]any
}

func ListSessions(directory string, limit *int, offset int, includeWorktrees bool) []shared.SDKSessionInfo {
	var projectDirs []string
	if directory != "" {
		projectDirs = projectDirsForDirectory(directory, includeWorktrees)
	} else {
		projectDirs = allProjectDirs()
	}

	bySessionID := make(map[string]shared.SDKSessionInfo)
	for _, projectDir := range projectDirs {
		matches, err := filepath.Glob(filepath.Join(projectDir, "*.jsonl"))
		if err != nil {
			continue
		}
		for _, filePath := range matches {
			sessionID := strings.TrimSuffix(filepath.Base(filePath), ".jsonl")
			if !validateUUID(sessionID) {
				continue
			}
			info := parseSessionInfo(filePath, sessionID)
			if info == nil {
				continue
			}
			existing, ok := bySessionID[sessionID]
			if !ok || info.LastModified > existing.LastModified {
				bySessionID[sessionID] = *info
			}
		}
	}

	sessions := make([]shared.SDKSessionInfo, 0, len(bySessionID))
	for _, info := range bySessionID {
		sessions = append(sessions, info)
	}

	sort.Slice(sessions, func(i, j int) bool {
		if sessions[i].LastModified == sessions[j].LastModified {
			return sessions[i].SessionID < sessions[j].SessionID
		}
		return sessions[i].LastModified > sessions[j].LastModified
	})

	if offset > 0 {
		if offset >= len(sessions) {
			return []shared.SDKSessionInfo{}
		}
		sessions = sessions[offset:]
	}
	if limit != nil && *limit > 0 && *limit < len(sessions) {
		sessions = sessions[:*limit]
	}
	return sessions
}

func GetSessionInfo(sessionID string, directory string) *shared.SDKSessionInfo {
	if !validateUUID(sessionID) {
		return nil
	}
	path, _ := findSessionFile(sessionID, directory)
	if path == "" {
		return nil
	}
	return parseSessionInfo(path, sessionID)
}

func GetSessionMessages(sessionID string, directory string, limit *int, offset int) []shared.SessionMessage {
	if !validateUUID(sessionID) {
		return []shared.SessionMessage{}
	}
	path, _ := findSessionFile(sessionID, directory)
	if path == "" {
		return []shared.SessionMessage{}
	}

	entries, err := readTranscriptEntries(path)
	if err != nil {
		return []shared.SessionMessage{}
	}

	chain := buildConversationChain(entries)
	messages := make([]shared.SessionMessage, 0, len(chain))
	for _, entry := range chain {
		if !isVisibleMessage(entry) {
			continue
		}
		messages = append(messages, shared.SessionMessage{
			Type:            entry.Type,
			UUID:            entry.UUID,
			SessionID:       entry.SessionID,
			Message:         entry.Message,
			ParentToolUseID: nil,
		})
	}

	if offset > 0 {
		if offset >= len(messages) {
			return []shared.SessionMessage{}
		}
		messages = messages[offset:]
	}
	if limit != nil && *limit > 0 && *limit < len(messages) {
		messages = messages[:*limit]
	}
	return messages
}

func RenameSession(sessionID, title, directory string) error {
	if !validateUUID(sessionID) {
		return fmt.Errorf("invalid session_id: %s", sessionID)
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return fmt.Errorf("title must be non-empty")
	}
	return appendToSession(sessionID, compactJSON(map[string]any{
		"type":        "custom-title",
		"customTitle": title,
		"sessionId":   sessionID,
	})+"\n", directory)
}

func TagSession(sessionID string, tag *string, directory string) error {
	if !validateUUID(sessionID) {
		return fmt.Errorf("invalid session_id: %s", sessionID)
	}
	tagValue := ""
	if tag != nil {
		tagValue = strings.TrimSpace(sanitizeUnicode(*tag))
		if tagValue == "" {
			return fmt.Errorf("tag must be non-empty (use nil to clear)")
		}
	}
	return appendToSession(sessionID, compactJSON(map[string]any{
		"type":      "tag",
		"tag":       tagValue,
		"sessionId": sessionID,
	})+"\n", directory)
}

func DeleteSession(sessionID, directory string) error {
	if !validateUUID(sessionID) {
		return fmt.Errorf("invalid session_id: %s", sessionID)
	}
	path, _ := findSessionFile(sessionID, directory)
	if path == "" {
		return fmt.Errorf("session %s not found", sessionID)
	}
	if err := os.Remove(path); err != nil {
		return err
	}
	// Subagent transcripts live in a sibling {sessionID}/ dir; often absent.
	siblingDir := filepath.Join(filepath.Dir(path), sessionID)
	_ = os.RemoveAll(siblingDir)
	return nil
}

func ForkSession(sessionID, directory string, upToMessageID, title *string) (*shared.ForkSessionResult, error) {
	if !validateUUID(sessionID) {
		return nil, fmt.Errorf("invalid session_id: %s", sessionID)
	}
	if upToMessageID != nil && !validateUUID(*upToMessageID) {
		return nil, fmt.Errorf("invalid up_to_message_id: %s", *upToMessageID)
	}

	sourcePath, projectDir := findSessionFile(sessionID, directory)
	if sourcePath == "" {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}

	rawEntries, err := readRawEntries(sourcePath)
	if err != nil {
		return nil, err
	}
	if len(rawEntries) == 0 {
		return nil, fmt.Errorf("session has no messages to fork")
	}

	transcript := make([]map[string]any, 0, len(rawEntries))
	contentReplacements := make([]any, 0)
	for _, entry := range rawEntries {
		entryType, _ := entry["type"].(string)
		uuid, _ := entry["uuid"].(string)
		if transcriptTypes[entryType] && uuid != "" {
			if isSidechain(entry) {
				continue
			}
			transcript = append(transcript, entry)
			continue
		}
		if entryType == "content-replacement" {
			entrySessionID, _ := entry["sessionId"].(string)
			if entrySessionID == sessionID {
				if replacements, ok := entry["replacements"].([]any); ok {
					contentReplacements = append(contentReplacements, replacements...)
				}
			}
		}
	}
	if len(transcript) == 0 {
		return nil, fmt.Errorf("session has no messages to fork")
	}

	if upToMessageID != nil {
		cutIndex := -1
		for i, entry := range transcript {
			if uuid, _ := entry["uuid"].(string); uuid == *upToMessageID {
				cutIndex = i
				break
			}
		}
		if cutIndex == -1 {
			return nil, fmt.Errorf("up_to_message_id not found: %s", *upToMessageID)
		}
		transcript = transcript[:cutIndex+1]
	}

	uuidMap := make(map[string]string, len(transcript))
	for _, entry := range transcript {
		uuid := entry["uuid"].(string)
		uuidMap[uuid] = newUUID()
	}

	writable := make([]map[string]any, 0, len(transcript))
	byUUID := make(map[string]map[string]any, len(transcript))
	for _, entry := range transcript {
		uuid := entry["uuid"].(string)
		byUUID[uuid] = entry
		if entry["type"] != "progress" {
			writable = append(writable, entry)
		}
	}
	if len(writable) == 0 {
		return nil, fmt.Errorf("session has no messages to fork")
	}

	forkedSessionID := newUUID()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	lines := make([]string, 0, len(writable)+2)
	for i, original := range writable {
		copied := deepCopyMap(original)
		originalUUID, _ := original["uuid"].(string)
		copied["uuid"] = uuidMap[originalUUID]

		parentUUID, _ := original["parentUuid"].(string)
		var newParent any
		for parentUUID != "" {
			parentEntry := byUUID[parentUUID]
			if parentEntry == nil {
				break
			}
			if parentType, _ := parentEntry["type"].(string); parentType != "progress" {
				newParent = uuidMap[parentUUID]
				break
			}
			parentUUID, _ = parentEntry["parentUuid"].(string)
		}
		copied["parentUuid"] = newParent

		if logicalParent, _ := original["logicalParentUuid"].(string); logicalParent != "" {
			if remapped := uuidMap[logicalParent]; remapped != "" {
				copied["logicalParentUuid"] = remapped
			}
		}

		if i == len(writable)-1 {
			copied["timestamp"] = now
		}
		copied["sessionId"] = forkedSessionID
		delete(copied, "session_id")
		copied["isSidechain"] = false
		copied["forkedFrom"] = map[string]any{
			"sessionId":   sessionID,
			"messageUuid": originalUUID,
		}
		delete(copied, "teamName")
		delete(copied, "agentName")
		delete(copied, "slug")
		delete(copied, "sourceToolAssistantUUID")
		lines = append(lines, compactJSON(copied))
	}

	if len(contentReplacements) > 0 {
		lines = append(lines, compactJSON(map[string]any{
			"type":         "content-replacement",
			"sessionId":    forkedSessionID,
			"replacements": contentReplacements,
		}))
	}

	forkTitle := ""
	if title != nil {
		forkTitle = strings.TrimSpace(*title)
	}
	if forkTitle == "" {
		base := deriveForkBaseTitle(rawEntries)
		if base == "" {
			base = "Forked session"
		}
		forkTitle = base + " (fork)"
	}
	lines = append(lines, compactJSON(map[string]any{
		"type":        "custom-title",
		"customTitle": forkTitle,
		"sessionId":   forkedSessionID,
	}))

	targetPath := filepath.Join(projectDir, forkedSessionID+".jsonl")
	file, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	if _, err := file.WriteString(strings.Join(lines, "\n") + "\n"); err != nil {
		return nil, err
	}

	return &shared.ForkSessionResult{SessionID: forkedSessionID}, nil
}

func parseSessionInfo(filePath, sessionID string) *shared.SDKSessionInfo {
	stat, err := os.Stat(filePath)
	if err != nil || stat.Size() == 0 {
		return nil
	}

	rawEntries, err := readRawEntries(filePath)
	if err != nil || len(rawEntries) == 0 {
		return nil
	}
	if isSidechain(rawEntries[0]) {
		return nil
	}

	var (
		summary     string
		customTitle *string
		aiTitle     *string
		lastPrompt  *string
		firstPrompt *string
		commandHint string
		gitBranch   *string
		cwd         *string
		tag         *string
		createdAt   *int64
	)

	for _, entry := range rawEntries {
		if createdAt == nil {
			if ts := parseEntryTimestamp(entry); ts != nil {
				createdAt = ts
			}
		}
		if value, ok := entry["gitBranch"].(string); ok && value != "" {
			gitBranch = &value
		}
		if value, ok := entry["cwd"].(string); ok && value != "" {
			cwd = &value
		}

		switch entryType, _ := entry["type"].(string); entryType {
		case "summary":
			if value, ok := entry["summary"].(string); ok && strings.TrimSpace(value) != "" {
				summary = strings.TrimSpace(value)
			}
			if value, ok := entry["customTitle"].(string); ok && strings.TrimSpace(value) != "" {
				v := strings.TrimSpace(value)
				customTitle = &v
			}
			if value, ok := entry["aiTitle"].(string); ok && strings.TrimSpace(value) != "" {
				v := strings.TrimSpace(value)
				aiTitle = &v
			}
			if value, ok := entry["lastPrompt"].(string); ok && strings.TrimSpace(value) != "" {
				v := strings.TrimSpace(value)
				lastPrompt = &v
			}
		case "custom-title":
			if value, ok := entry["customTitle"].(string); ok && strings.TrimSpace(value) != "" {
				v := strings.TrimSpace(value)
				customTitle = &v
			}
			if value, ok := entry["aiTitle"].(string); ok && strings.TrimSpace(value) != "" {
				v := strings.TrimSpace(value)
				aiTitle = &v
			}
			if value, ok := entry["lastPrompt"].(string); ok && strings.TrimSpace(value) != "" {
				v := strings.TrimSpace(value)
				lastPrompt = &v
			}
		case "tag":
			if value, ok := entry["tag"].(string); ok {
				if strings.TrimSpace(value) == "" {
					tag = nil
				} else {
					v := strings.TrimSpace(value)
					tag = &v
				}
			}
		case shared.MessageTypeUser:
			if firstPrompt == nil {
				prompt, fallback := extractPrompt(entry)
				if prompt != "" {
					p := prompt
					firstPrompt = &p
				} else if commandHint == "" {
					commandHint = fallback
				}
			}
		}
	}

	if customTitle != nil {
		summary = *customTitle
	} else if aiTitle != nil {
		summary = *aiTitle
	} else if lastPrompt != nil {
		summary = *lastPrompt
	}
	if summary == "" {
		if firstPrompt != nil {
			summary = *firstPrompt
		} else if commandHint != "" {
			summary = commandHint
		}
	}
	if summary == "" {
		return nil
	}

	fileSize := stat.Size()
	return &shared.SDKSessionInfo{
		SessionID:    sessionID,
		Summary:      summary,
		LastModified: stat.ModTime().UnixMilli(),
		FileSize:     &fileSize,
		CustomTitle:  firstNonNilString(customTitle, aiTitle),
		FirstPrompt:  firstPrompt,
		GitBranch:    gitBranch,
		Cwd:          cwd,
		Tag:          tag,
		CreatedAt:    createdAt,
	}
}

func deriveForkBaseTitle(rawEntries []map[string]any) string {
	var (
		customTitle *string
		aiTitle     *string
		firstPrompt *string
	)

	for _, entry := range rawEntries {
		if isSidechain(entry) {
			continue
		}
		if value, ok := entry["customTitle"].(string); ok && strings.TrimSpace(value) != "" {
			v := strings.TrimSpace(value)
			customTitle = &v
		}
		if value, ok := entry["aiTitle"].(string); ok && strings.TrimSpace(value) != "" {
			v := strings.TrimSpace(value)
			aiTitle = &v
		}
		if firstPrompt == nil {
			if entryType, _ := entry["type"].(string); entryType == shared.MessageTypeUser {
				prompt, _ := extractPrompt(entry)
				if prompt != "" {
					p := prompt
					firstPrompt = &p
				}
			}
		}
	}

	if value := firstNonNilString(customTitle, aiTitle, firstPrompt); value != nil {
		return *value
	}
	return ""
}

func firstNonNilString(values ...*string) *string {
	for _, value := range values {
		if value != nil && strings.TrimSpace(*value) != "" {
			return value
		}
	}
	return nil
}

func extractPrompt(entry map[string]any) (string, string) {
	if isMeta, _ := entry["isMeta"].(bool); isMeta {
		return "", ""
	}
	message, ok := entry["message"].(map[string]any)
	if !ok {
		return "", ""
	}
	content := message["content"]
	switch value := content.(type) {
	case string:
		prompt := strings.TrimSpace(value)
		if prompt == "" || skipPromptRE.MatchString(prompt) {
			if command := extractCommandName(prompt); command != "" {
				return "", command
			}
			return "", ""
		}
		return truncatePrompt(prompt), extractCommandName(prompt)
	case []any:
		hasToolResult := false
		var texts []string
		for _, item := range value {
			block, ok := item.(map[string]any)
			if !ok {
				continue
			}
			blockType, _ := block["type"].(string)
			if blockType == "tool_result" {
				hasToolResult = true
				break
			}
			if blockType == "text" {
				if text, ok := block["text"].(string); ok && strings.TrimSpace(text) != "" {
					texts = append(texts, strings.TrimSpace(text))
				}
			}
		}
		if hasToolResult || len(texts) == 0 {
			return "", ""
		}
		prompt := strings.Join(texts, "\n")
		return truncatePrompt(prompt), extractCommandName(prompt)
	default:
		return "", ""
	}
}

func findSessionFile(sessionID, directory string) (string, string) {
	fileName := sessionID + ".jsonl"
	candidates := allProjectDirs()
	if directory != "" {
		candidates = projectDirsForDirectory(directory, true)
	}
	for _, projectDir := range candidates {
		path := filepath.Join(projectDir, fileName)
		info, err := os.Stat(path)
		if err == nil && !info.IsDir() && info.Size() > 0 {
			return path, projectDir
		}
	}
	return "", ""
}

func appendToSession(sessionID, data, directory string) error {
	path, _ := findSessionFile(sessionID, directory)
	if path == "" {
		return fmt.Errorf("session %s not found", sessionID)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(data)
	return err
}

func projectDirsForDirectory(directory string, includeWorktrees bool) []string {
	canonical := canonicalizePath(directory)
	candidates := make([]string, 0)
	if projectDir := findProjectDir(canonical); projectDir != "" {
		candidates = append(candidates, projectDir)
	}
	if includeWorktrees {
		for _, worktree := range getWorktreePaths(canonical) {
			if projectDir := findProjectDir(worktree); projectDir != "" {
				candidates = append(candidates, projectDir)
			}
		}
	}
	return dedupeStrings(candidates)
}

func allProjectDirs() []string {
	projectsDir := filepath.Join(getClaudeConfigHomeDir(), "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return nil
	}
	dirs := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, filepath.Join(projectsDir, entry.Name()))
		}
	}
	sort.Strings(dirs)
	return dirs
}

func findProjectDir(projectPath string) string {
	exact := filepath.Join(getClaudeConfigHomeDir(), "projects", sanitizePath(projectPath))
	if info, err := os.Stat(exact); err == nil && info.IsDir() {
		return exact
	}
	sanitized := sanitizePath(projectPath)
	if len(sanitized) <= maxSanitizedLength {
		return ""
	}
	prefix := sanitized[:maxSanitizedLength] + "-"
	for _, dir := range allProjectDirs() {
		if strings.HasPrefix(filepath.Base(dir), prefix) {
			return dir
		}
	}
	return ""
}

func getClaudeConfigHomeDir() string {
	if configDir := os.Getenv("CLAUDE_CONFIG_DIR"); configDir != "" {
		return normalizeNFC(configDir)
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return normalizeNFC(".claude")
	}
	return normalizeNFC(filepath.Join(homeDir, ".claude"))
}

func getWorktreePaths(directory string) []string {
	cmd := exec.Command("git", "-C", directory, "worktree", "list", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return nil
	}
	paths := make([]string, 0)
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "worktree ") {
			paths = append(paths, canonicalizePath(strings.TrimPrefix(line, "worktree ")))
		}
	}
	return dedupeStrings(paths)
}

func readTranscriptEntries(path string) ([]transcriptEntry, error) {
	rawEntries, err := readRawEntries(path)
	if err != nil {
		return nil, err
	}
	entries := make([]transcriptEntry, 0, len(rawEntries))
	for _, raw := range rawEntries {
		entryType, _ := raw["type"].(string)
		message, _ := raw["message"].(map[string]any)
		uuid, _ := raw["uuid"].(string)
		sessionID, _ := raw["sessionId"].(string)
		if sessionID == "" {
			sessionID, _ = raw["session_id"].(string)
		}
		var parentToolUseID *string
		if value, ok := raw["parent_tool_use_id"].(string); ok {
			parentToolUseID = &value
		}
		var parentUUID *string
		if value, ok := raw["parentUuid"].(string); ok {
			parentUUID = &value
		}
		if !transcriptTypes[entryType] || uuid == "" {
			continue
		}
		entries = append(entries, transcriptEntry{
			Type:            entryType,
			UUID:            uuid,
			SessionID:       sessionID,
			Message:         message,
			ParentToolUseID: parentToolUseID,
			ParentUUID:      parentUUID,
			Raw:             raw,
		})
	}
	return entries, nil
}

func readRawEntries(path string) ([]map[string]any, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	entries := make([]map[string]any, 0)
	scanner := bufio.NewScanner(file)
	buffer := make([]byte, 0, 1024*1024)
	scanner.Buffer(buffer, 16*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

func isSidechain(entry map[string]any) bool {
	sidechain, _ := entry["isSidechain"].(bool)
	return sidechain
}

func parseEntryTimestamp(entry map[string]any) *int64 {
	for _, key := range []string{"timestamp", "createdAt"} {
		if value, ok := entry[key].(string); ok && value != "" {
			if t, err := time.Parse(time.RFC3339Nano, value); err == nil {
				ms := t.UnixMilli()
				return &ms
			}
		}
	}
	return nil
}

func truncatePrompt(prompt string) string {
	runes := []rune(prompt)
	if len(runes) <= 200 {
		return prompt
	}
	return string(runes[:200]) + "..."
}

func extractCommandName(text string) string {
	matches := commandNameRE.FindStringSubmatch(text)
	if len(matches) == 2 {
		return matches[1]
	}
	return ""
}

func compactJSON(value any) string {
	data, _ := json.Marshal(value)
	return string(data)
}

func deepCopyMap(in map[string]any) map[string]any {
	data, _ := json.Marshal(in)
	out := make(map[string]any)
	_ = json.Unmarshal(data, &out)
	return out
}

func buildConversationChain(entries []transcriptEntry) []transcriptEntry {
	if len(entries) == 0 {
		return nil
	}

	byUUID := make(map[string]transcriptEntry, len(entries))
	entryIndex := make(map[string]int, len(entries))
	parentUUIDs := make(map[string]bool)
	for i, entry := range entries {
		if entry.UUID == "" {
			continue
		}
		byUUID[entry.UUID] = entry
		entryIndex[entry.UUID] = i
		if entry.ParentUUID != nil && *entry.ParentUUID != "" {
			parentUUIDs[*entry.ParentUUID] = true
		}
	}

	terminals := make([]transcriptEntry, 0)
	for _, entry := range entries {
		if entry.UUID != "" && !parentUUIDs[entry.UUID] {
			terminals = append(terminals, entry)
		}
	}

	leaves := make([]transcriptEntry, 0)
	for _, terminal := range terminals {
		current := terminal
		seen := make(map[string]bool)
		for current.UUID != "" && !seen[current.UUID] {
			seen[current.UUID] = true
			if current.Type == shared.MessageTypeUser || current.Type == shared.MessageTypeAssistant {
				leaves = append(leaves, current)
				break
			}
			if current.ParentUUID == nil || *current.ParentUUID == "" {
				break
			}
			parent, ok := byUUID[*current.ParentUUID]
			if !ok {
				break
			}
			current = parent
		}
	}
	if len(leaves) == 0 {
		return nil
	}

	bestLeaf := leaves[0]
	bestIndex := -1
	for _, leaf := range leaves {
		if !isPreferredLeaf(leaf) {
			continue
		}
		if idx := entryIndex[leaf.UUID]; idx > bestIndex {
			bestLeaf = leaf
			bestIndex = idx
		}
	}
	if bestIndex == -1 {
		bestIndex = entryIndex[bestLeaf.UUID]
		for _, leaf := range leaves[1:] {
			if idx := entryIndex[leaf.UUID]; idx > bestIndex {
				bestLeaf = leaf
				bestIndex = idx
			}
		}
	}

	chain := make([]transcriptEntry, 0)
	current := bestLeaf
	seen := make(map[string]bool)
	for current.UUID != "" && !seen[current.UUID] {
		seen[current.UUID] = true
		chain = append(chain, current)
		if current.ParentUUID == nil || *current.ParentUUID == "" {
			break
		}
		parent, ok := byUUID[*current.ParentUUID]
		if !ok {
			break
		}
		current = parent
	}

	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain
}

func isVisibleMessage(entry transcriptEntry) bool {
	if entry.Type != shared.MessageTypeUser && entry.Type != shared.MessageTypeAssistant {
		return false
	}
	if boolValue(entry.Raw, "isMeta") || boolValue(entry.Raw, "isSidechain") {
		return false
	}
	teamName, _ := entry.Raw["teamName"].(string)
	return teamName == ""
}

func isPreferredLeaf(entry transcriptEntry) bool {
	if boolValue(entry.Raw, "isSidechain") || boolValue(entry.Raw, "isMeta") {
		return false
	}
	teamName, _ := entry.Raw["teamName"].(string)
	return teamName == ""
}

func boolValue(values map[string]any, key string) bool {
	value, _ := values[key].(bool)
	return value
}

func sanitizePath(name string) string {
	sanitized := sanitizeRE.ReplaceAllString(name, "-")
	if len(sanitized) <= maxSanitizedLength {
		return sanitized
	}
	hash := simpleHash(name)
	return sanitized[:maxSanitizedLength] + "-" + hash
}

func simpleHash(s string) string {
	h := int32(0)
	for _, ch := range s {
		h = h*31 + int32(ch)
	}
	if h < 0 {
		h = -h
	}
	return strings.ToLower(strconvBase36(int64(h)))
}

func strconvBase36(v int64) string {
	const digits = "0123456789abcdefghijklmnopqrstuvwxyz"
	if v == 0 {
		return "0"
	}
	out := make([]byte, 0)
	for v > 0 {
		out = append(out, digits[v%36])
		v /= 36
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return string(out)
}

func canonicalizePath(path string) string {
	if path == "" {
		return path
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	if abs, err := filepath.Abs(path); err == nil {
		return normalizeNFC(abs)
	}
	return normalizeNFC(path)
}

func validateUUID(value string) bool {
	return uuidRE.MatchString(value)
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func sanitizeUnicode(value string) string {
	current := value
	for i := 0; i < 10; i++ {
		previous := current
		current = strings.Map(func(r rune) rune {
			if unicode.Is(unicode.Cf, r) || unicode.Is(unicode.Co, r) || unicode.Is(unicode.Cn, r) {
				return -1
			}
			switch {
			case r >= 0x200b && r <= 0x200f:
				return -1
			case r >= 0x202a && r <= 0x202e:
				return -1
			case r >= 0x2066 && r <= 0x2069:
				return -1
			case r == 0xfeff:
				return -1
			case r >= 0xe000 && r <= 0xf8ff:
				return -1
			default:
				return r
			}
		}, current)
		if current == previous {
			break
		}
	}
	return current
}

func normalizeNFC(value string) string {
	return norm.NFC.String(value)
}

func newUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf(
		"%08x-%04x-%04x-%04x-%012x",
		b[0:4],
		b[4:6],
		b[6:8],
		b[8:10],
		b[10:16],
	)
}
