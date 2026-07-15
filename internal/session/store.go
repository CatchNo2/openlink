package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Turn 单条对话轮次内容。
type Turn struct {
	Role    string `json:"role"` // user | assistant | tool
	Content string `json:"content"`
	TS      int64  `json:"ts"`
}

// Session 一次会话（按浏览器会话/标签页维度）。
type Session struct {
	ID           string `json:"id"`
	Turns        []Turn `json:"turns"`
	RoundCount   int    `json:"round_count"`
	LastActivity int64  `json:"last_activity"`
	LastReviewed int64  `json:"last_reviewed"`
}

const (
	maxTurnsPerSession = 240
	maxTurnChars       = 6000
)

// Store 会话存储（内存 + 防抖落盘）。
type Store struct {
	rootDir  string
	mu       sync.RWMutex
	sessions map[string]*Session
	saveTicker *time.Ticker
	stopCh   chan struct{}
}

var (
	store *Store
	once  sync.Once
)

func pathOf(rootDir string) string { return filepath.Join(rootDir, ".openlink", "sessions.json") }

// Init 初始化会话存储单例，并后台防抖落盘。
func Init(rootDir string) {
	once.Do(func() {
		s := &Store{
			rootDir:  rootDir,
			sessions: map[string]*Session{},
			stopCh:   make(chan struct{}),
		}
		if data, err := os.ReadFile(pathOf(rootDir)); err == nil {
			_ = json.Unmarshal(data, &s.sessions)
		}
		_ = os.MkdirAll(filepath.Join(rootDir, ".openlink"), 0755)
		store = s
		go s.backgroundSave()
	})
}

// Get 获取单例。
func Get() *Store {
	if store == nil {
		Init(".")
	}
	return store
}

// stop 进程退出时调用（可选）。
func (s *Store) stop() {
	select {
	case <-s.stopCh:
	default:
		close(s.stopCh)
	}
}

// backgroundSave 每 15 秒落盘一次。
func (s *Store) backgroundSave() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopCh:
			s.save()
			return
		case <-ticker.C:
			s.save()
		}
	}
}

func (s *Store) save() {
	s.mu.RLock()
	data, err := json.MarshalIndent(s.sessions, "", "  ")
	s.mu.RUnlock()
	if err != nil {
		return
	}
	_ = os.WriteFile(pathOf(s.rootDir), data, 0644)
}

// LogTurn 记录一条对话轮次，更新最后活动时间与轮次计数。
func (s *Store) LogTurn(sessionID, role, content string) {
	if strings.TrimSpace(sessionID) == "" {
		sessionID = "default"
	}
	content = strings.TrimSpace(content)
	if len(content) > maxTurnChars {
		content = content[:maxTurnChars] + "...(已截断)"
	}
	now := time.Now().Unix()
	s.mu.Lock()
	ses, ok := s.sessions[sessionID]
	if !ok {
		ses = &Session{ID: sessionID, LastActivity: now}
		s.sessions[sessionID] = ses
	}
	ses.Turns = append(ses.Turns, Turn{Role: role, Content: content, TS: now})
	if len(ses.Turns) > maxTurnsPerSession {
		ses.Turns = ses.Turns[len(ses.Turns)-maxTurnsPerSession:]
	}
	if role == "user" {
		ses.RoundCount++
	}
	ses.LastActivity = now
	s.mu.Unlock()
}

// RoundCount 返回某会话的轮次。
func (s *Store) RoundCount(sessionID string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if ses, ok := s.sessions[sessionID]; ok {
		return ses.RoundCount
	}
	return 0
}

// LastActivity 返回某会话最后活动时间（unix 秒）。
func (s *Store) LastActivity(sessionID string) int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if ses, ok := s.sessions[sessionID]; ok {
		return ses.LastActivity
	}
	return 0
}

// MarkReviewed 标记某会话最近一次复盘时间。
func (s *Store) MarkReviewed(sessionID string) {
	s.mu.Lock()
	if ses, ok := s.sessions[sessionID]; ok {
		ses.LastReviewed = time.Now().Unix()
	}
	s.mu.Unlock()
}

// MostActiveID 返回当前活动最频繁的会话 ID；无则返回空。
func (s *Store) MostActiveID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var best string
	var bestTS int64
	for id, ses := range s.sessions {
		if ses.LastActivity > bestTS {
			bestTS = ses.LastActivity
			best = id
		}
	}
	return best
}

// Transcript 返回最近 n 条轮次的纯文本（用于 LLM 复盘）。
func (s *Store) Transcript(sessionID string, n int) string {
	s.mu.RLock()
	ses, ok := s.sessions[sessionID]
	s.mu.RUnlock()
	if !ok {
		return ""
	}
	turns := ses.Turns
	if n > 0 && len(turns) > n {
		turns = turns[len(turns)-n:]
	}
	var sb strings.Builder
	for _, t := range turns {
		role := strings.ToUpper(t.Role)
		sb.WriteString(fmt.Sprintf("[%s] %s\n", role, t.Content))
	}
	return sb.String()
}
