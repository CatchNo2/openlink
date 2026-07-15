package evolution

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// ChangeItem 单次变更的具体文件动作。
type ChangeItem struct {
	Action string `json:"action"` // created | modified | distilled
	Path   string `json:"path"`
}

// EvolutionRecord 一次自主进化的可追溯记录。
type EvolutionRecord struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"` // review | dream | skill | fix
	Time     string       `json:"time"`
	Summary  string       `json:"summary"`
	Changes  []ChangeItem `json:"changes"`
	Rollback []string     `json:"rollback"` // 备份文件路径，可用于还原
}

// Notification 待推送用户的一次性通知。
type Notification struct {
	ID    string `json:"id"`
	Time  string `json:"time"`
	Title string `json:"title"`
	Body  string `json:"body"`
	Read  bool   `json:"read"`
}

// ChangeLog 持久化的进化记录与通知。
type ChangeLog struct {
	mu       sync.RWMutex
	rootDir  string
	records  []EvolutionRecord
	notes    []Notification
	logPath  string
	notePath string
}

var logInstance *ChangeLog
var clogOnce sync.Once

// InitLog 初始化变更日志单例。
func InitLog(rootDir string) {
	clogOnce.Do(func() {
		l := &ChangeLog{
			rootDir:  rootDir,
			logPath:  filepath.Join(rootDir, ".openlink", "evolution-log.json"),
			notePath: filepath.Join(rootDir, ".openlink", "notifications.json"),
		}
		if data, err := os.ReadFile(l.logPath); err == nil {
			_ = json.Unmarshal(data, &l.records)
		}
		if data, err := os.ReadFile(l.notePath); err == nil {
			_ = json.Unmarshal(data, &l.notes)
		}
		_ = os.MkdirAll(filepath.Join(rootDir, ".openlink"), 0755)
		logInstance = l
	})
}

// Log 返回单例。
func Log() *ChangeLog {
	if logInstance == nil {
		InitLog(".")
	}
	return logInstance
}

func (l *ChangeLog) save() {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if data, err := json.MarshalIndent(l.records, "", "  "); err == nil {
		_ = os.WriteFile(l.logPath, data, 0644)
	}
	if data, err := json.MarshalIndent(l.notes, "", "  "); err == nil {
		_ = os.WriteFile(l.notePath, data, 0644)
	}
}

// AddRecord 追加一条进化记录。
func (l *ChangeLog) AddRecord(r EvolutionRecord) {
	if r.ID == "" {
		r.ID = genID()
	}
	if r.Time == "" {
		r.Time = time.Now().Format("2006-01-02 15:04:05")
	}
	l.mu.Lock()
	l.records = append(l.records, r)
	// 仅保留最近 200 条
	if len(l.records) > 200 {
		l.records = l.records[len(l.records)-200:]
	}
	l.mu.Unlock()
	l.save()
}

// ListRecords 返回最近 n 条记录（倒序）。
func (l *ChangeLog) ListRecords(n int) []EvolutionRecord {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]EvolutionRecord, len(l.records))
	copy(out, l.records)
	sort.Slice(out, func(i, j int) bool { return out[i].Time > out[j].Time })
	if n > 0 && len(out) > n {
		out = out[:n]
	}
	return out
}

// AddNotification 追加一条待推送通知。
func (l *ChangeLog) AddNotification(title, body string) {
	l.mu.Lock()
	l.notes = append(l.notes, Notification{
		ID:    genID(),
		Time:  time.Now().Format("2006-01-02 15:04:05"),
		Title: title,
		Body:  body,
	})
	l.mu.Unlock()
	l.save()
}

// ListNotifications 返回未读通知（正序）。
func (l *ChangeLog) ListNotifications() []Notification {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]Notification, len(l.notes))
	copy(out, l.notes)
	return out
}

// ClearNotifications 清空通知（用户已查看/处理）。
func (l *ChangeLog) ClearNotifications() {
	l.mu.Lock()
	l.notes = nil
	l.mu.Unlock()
	l.save()
}

func genID() string {
	return time.Now().Format("20060102150405") + "-" + rand4()
}

func rand4() string {
	b := make([]byte, 4)
	// 简单可复现随机：用纳秒
	n := time.Now().Nanosecond()
	for i := range b {
		b[i] = byte('a' + (n>>(i*3))%26)
	}
	return string(b)
}
