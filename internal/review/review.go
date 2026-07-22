package review

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// 超过此行数则跳过 diff 计算，避免大文件拖垮内存
const maxDiffLines = 10000

// FileSnapshot 文件修改前的快照
type FileSnapshot struct {
	Path    string `json:"path"`
	Content []byte `json:"-"`
	Exists  bool   `json:"exists"`
	Mode    os.FileMode `json:"-"`
}

// DiffLine 单行差异
type DiffLine struct {
	Type string `json:"type"` // add / del / ctx
	Text string `json:"text"`
}

// FileChange 文件变更记录
type FileChange struct {
	Path    string    `json:"path"`
	Action  string    `json:"action"` // created / modified / deleted
	Source  string    `json:"source"` // 触发来源（工具名）
	Time    time.Time `json:"time"`
	HasDiff bool      `json:"hasDiff"`
	Diff    []DiffLine `json:"diff,omitempty"`
}

// ReviewSession 一次任务的审查会话，累积多次文件操作
type ReviewSession struct {
	mu        sync.RWMutex
	Snapshots map[string]*FileSnapshot
	Changes   []FileChange
}

// Manager 审查管理器（全局单例）
type Manager struct {
	mu      sync.RWMutex
	session *ReviewSession
}

// NewManager 创建审查管理器
func NewManager() *Manager {
	return &Manager{}
}

// Snapshot 在文件被修改前捕获快照
func (m *Manager) Snapshot(path, source string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.session == nil {
		m.session = &ReviewSession{
			Snapshots: make(map[string]*FileSnapshot),
		}
	}

	m.session.mu.Lock()
	defer m.session.mu.Unlock()

	// 同一文件只快照一次
	if _, exists := m.session.Snapshots[path]; exists {
		return
	}

	snap := &FileSnapshot{Path: path, Mode: 0644}
	data, err := os.ReadFile(path)
	if err == nil {
		snap.Content = data
		snap.Exists = true
	}
	// 获取文件权限
	if info, err := os.Stat(path); err == nil {
		snap.Mode = info.Mode()
	}

	m.session.Snapshots[path] = snap
}

// RecordChange 文件修改后记录变更
func (m *Manager) RecordChange(path, source string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.session == nil {
		return
	}

	m.session.mu.Lock()
	defer m.session.mu.Unlock()

	snap, hasSnapshot := m.session.Snapshots[path]

	action := "modified"
	if !hasSnapshot || !snap.Exists {
		action = "created"
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		action = "deleted"
	}

	// 去重
	for _, c := range m.session.Changes {
		if c.Path == path {
			return
		}
	}

	m.session.Changes = append(m.session.Changes, FileChange{
		Path:   path,
		Action: action,
		Source: source,
		Time:   time.Now(),
	})
}

// Review 获取当前会话的变更列表（含来源、时间、行级 diff）
func (m *Manager) Review() []map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.session == nil {
		return nil
	}
	m.session.mu.RLock()
	defer m.session.mu.RUnlock()

	if len(m.session.Changes) == 0 {
		return nil
	}

	var result []map[string]interface{}
	for _, c := range m.session.Changes {
		item := map[string]interface{}{
			"path":   c.Path,
			"action": c.Action,
			"source": c.Source,
			"time":   c.Time.Format("15:04:05"),
		}
		// 计算行级 diff
		if diff, ok := m.computeDiff(c); ok {
			item["hasDiff"] = true
			item["diff"] = diff
		} else {
			item["hasDiff"] = false
		}
		result = append(result, item)
	}
	return result
}

// computeDiff 根据快照与当前文件内容计算行级差异
func (m *Manager) computeDiff(c FileChange) ([]DiffLine, bool) {
	snap, ok := m.session.Snapshots[c.Path]
	if !ok {
		return nil, false
	}

	var before, after []string
	if snap.Exists {
		before = strings.Split(string(snap.Content), "\n")
	}
	if c.Action != "deleted" {
		if data, err := os.ReadFile(c.Path); err == nil {
			after = strings.Split(string(data), "\n")
		}
	}

	// 大文件跳过
	if len(before)+len(after) > maxDiffLines {
		return nil, false
	}
	return diffLines(before, after), true
}

// HasSession 是否有未审查的变更
func (m *Manager) HasSession() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.session != nil && len(m.session.Changes) > 0
}

// Undo 撤回指定文件或全部文件
func (m *Manager) Undo(path string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.session == nil {
		return nil, fmt.Errorf("没有待审查的变更")
	}

	m.session.mu.Lock()
	defer m.session.mu.Unlock()

	if path == "" {
		// 撤回全部
		var restored []string
		for _, snap := range m.session.Snapshots {
			if err := restoreFile(snap); err != nil {
				return restored, err
			}
			restored = append(restored, snap.Path)
		}
		m.session = nil
		return restored, nil
	}

	// 撤回单个文件
	snap, exists := m.session.Snapshots[path]
	if !exists {
		return nil, fmt.Errorf("文件 %s 没有快照", path)
	}

	if err := restoreFile(snap); err != nil {
		return nil, err
	}

	// 从变更列表中移除
	var newChanges []FileChange
	for _, c := range m.session.Changes {
		if c.Path != path {
			newChanges = append(newChanges, c)
		}
	}
	m.session.Changes = newChanges
	delete(m.session.Snapshots, path)

	if len(m.session.Changes) == 0 {
		m.session = nil
	}

	return []string{path}, nil
}

// Keep 保留变更，清除快照
func (m *Manager) Keep(paths []string) []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.session == nil {
		return nil
	}

	m.session.mu.Lock()
	defer m.session.mu.Unlock()

	if len(paths) == 0 {
		// 保留全部
		var kept []string
		for _, c := range m.session.Changes {
			kept = append(kept, c.Path)
		}
		m.session = nil
		return kept
	}

	// 保留指定文件
	keepSet := make(map[string]bool)
	for _, p := range paths {
		keepSet[p] = true
	}

	var kept []string
	var remaining []FileChange
	for _, c := range m.session.Changes {
		if keepSet[c.Path] {
			kept = append(kept, c.Path)
			delete(m.session.Snapshots, c.Path)
		} else {
			remaining = append(remaining, c)
		}
	}
	m.session.Changes = remaining

	if len(m.session.Changes) == 0 {
		m.session = nil
	}

	return kept
}

// restoreFile 还原文件到快照状态
func restoreFile(snap *FileSnapshot) error {
	if !snap.Exists {
		// 文件之前不存在，删除它
		os.Remove(snap.Path)
		return nil
	}
	// 还原文件内容和权限
	return os.WriteFile(snap.Path, snap.Content, snap.Mode)
}

// diffLines 基于行 LCS 计算差异
func diffLines(a, b []string) []DiffLine {
	n, m := len(a), len(b)
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}
	var out []DiffLine
	i, j := 0, 0
	for i < n && j < m {
		if a[i] == b[j] {
			out = append(out, DiffLine{"ctx", a[i]})
			i++
			j++
		} else if dp[i+1][j] >= dp[i][j+1] {
			out = append(out, DiffLine{"del", a[i]})
			i++
		} else {
			out = append(out, DiffLine{"add", b[j]})
			j++
		}
	}
	for ; i < n; i++ {
		out = append(out, DiffLine{"del", a[i]})
	}
	for ; j < m; j++ {
		out = append(out, DiffLine{"add", b[j]})
	}
	return out
}
