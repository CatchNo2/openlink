package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Store 管理工作空间内的记忆与知识库文件。
//   - 核心记忆:  <rootDir>/MEMORY.md            （精炼、长期有效，注入系统提示）
//   - 天级记忆:  <rootDir>/memory/YYYY-MM-DD.md（按天、哈希去重）
//   - 知识库:    <rootDir>/knowledge/<topic>.md （Markdown 源文件，页面间交叉引用）
//   - 提示词:    <rootDir>/{AGENT,USER,RULE}.md
type Store struct {
	rootDir string
	mu      sync.RWMutex
	index   *DedupIndex
}

// DedupIndex 去重索引，持久化于 .openlink/memory-index.json。
type DedupIndex struct {
	CoreHashes  []string         `json:"core_hashes"`  // 核心记忆条目哈希
	DailyHashes []string         `json:"daily_hashes"` // 全部天级记忆条目哈希（跨天去重）
	ByDate      map[string][]string `json:"by_date"`    // 日期 -> 当天条目哈希
}

var (
	store *Store
	once  sync.Once
)

func indexPath(rootDir string) string {
	return filepath.Join(rootDir, ".openlink", "memory-index.json")
}

func loadIndex(rootDir string) *DedupIndex {
	idx := &DedupIndex{ByDate: map[string][]string{}}
	data, err := os.ReadFile(indexPath(rootDir))
	if err == nil {
		_ = json.Unmarshal(data, idx)
	}
	if idx.ByDate == nil {
		idx.ByDate = map[string][]string{}
	}
	return idx
}

// Init 初始化记忆存储单例。
func Init(rootDir string) {
	once.Do(func() {
		store = &Store{
			rootDir: rootDir,
			index:   loadIndex(rootDir),
		}
		// 确保目录存在
		_ = os.MkdirAll(filepath.Join(rootDir, "memory"), 0755)
		_ = os.MkdirAll(filepath.Join(rootDir, "knowledge"), 0755)
		_ = os.MkdirAll(filepath.Join(rootDir, ".openlink"), 0755)
		if _, err := os.Stat(filepath.Join(rootDir, "MEMORY.md")); os.IsNotExist(err) {
			_ = os.WriteFile(filepath.Join(rootDir, "MEMORY.md"), []byte("# MEMORY.md\n\n> 核心长期记忆，由 Agent 自动维护，保持精炼。\n"), 0644)
		}
	})
}

// Get 获取单例。
func Get() *Store {
	if store == nil {
		// 兜底：使用当前目录
		Init(".")
	}
	return store
}

func todayStr() string { return time.Now().Format("2006-01-02") }

func hashOf(s string) string {
	h := sha256.Sum256([]byte(strings.TrimSpace(s)))
	return hex.EncodeToString(h[:])
}

func (s *Store) persistIndex() {
	data, _ := json.MarshalIndent(s.index, "", "  ")
	_ = os.WriteFile(indexPath(s.rootDir), data, 0644)
}

func (s *Store) corePath() string    { return filepath.Join(s.rootDir, "MEMORY.md") }
func (s *Store) dailyPath(d string) string { return filepath.Join(s.rootDir, "memory", d+".md") }

// ---------- 核心记忆 ----------

// ReadCore 读取核心记忆全文。
func (s *Store) ReadCore() string {
	data, err := os.ReadFile(s.corePath())
	if err != nil {
		return ""
	}
	return string(data)
}

// AppendCore 追加一条核心记忆（按哈希去重）。返回是否实际写入。
func (s *Store) AppendCore(section string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	section = strings.TrimSpace(section)
	if section == "" {
		return false, fmt.Errorf("内容为空")
	}
	h := hashOf(section)
	for _, existing := range s.index.CoreHashes {
		if existing == h {
			return false, nil // 已存在
		}
	}
	f, err := os.OpenFile(s.corePath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return false, err
	}
	defer f.Close()
	if _, err := f.WriteString("\n" + section + "\n"); err != nil {
		return false, err
	}
	s.index.CoreHashes = append(s.index.CoreHashes, h)
	s.persistIndex()
	return true, nil
}

// SetCore 整体替换核心记忆（用于梦境蒸馏）。content 为完整文件内容。
func (s *Store) SetCore(content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.WriteFile(s.corePath(), []byte(content), 0644); err != nil {
		return err
	}
	// 重建哈希索引（按空行分段）
	parts := strings.Split(content, "\n")
	var hashes []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" && !strings.HasPrefix(p, "#") {
			hashes = append(hashes, hashOf(p))
		}
	}
	s.index.CoreHashes = hashes
	s.persistIndex()
	return nil
}

// ---------- 天级记忆 ----------

// ReadDaily 读取指定日期的天级记忆。
func (s *Store) ReadDaily(date string) string {
	if date == "" {
		date = todayStr()
	}
	data, err := os.ReadFile(s.dailyPath(date))
	if err != nil {
		return ""
	}
	return string(data)
}

// AppendDaily 向指定日期天级记忆追加一条（跨天哈希去重）。返回是否写入及原因。
func (s *Store) AppendDaily(date, entry string) (bool, error) {
	if date == "" {
		date = todayStr()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entry = strings.TrimSpace(entry)
	if entry == "" {
		return false, fmt.Errorf("内容为空")
	}
	h := hashOf(entry)
	for _, existing := range s.index.DailyHashes {
		if existing == h {
			return false, nil
		}
	}
	path := s.dailyPath(date)
	header := ""
	if _, err := os.Stat(path); os.IsNotExist(err) {
		header = fmt.Sprintf("# 天级记忆 %s\n\n", date)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return false, err
	}
	defer f.Close()
	block := fmt.Sprintf("\n## %s\n\n%s\n", time.Now().Format("15:04"), entry)
	if _, err := f.WriteString(header + block); err != nil {
		return false, err
	}
	s.index.DailyHashes = append(s.index.DailyHashes, h)
	s.index.ByDate[date] = append(s.index.ByDate[date], h)
	s.persistIndex()
	return true, nil
}

// CollectDaily 收集最近 n 天的天级记忆（按日期升序），返回 "日期: 内容" 拼接。
func (s *Store) CollectDaily(days int) string {
	if days < 1 {
		days = 1
	}
	var dates []string
	for i := days - 1; i >= 0; i-- {
		d := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		dates = append(dates, d)
	}
	var sb strings.Builder
	for _, d := range dates {
		content := s.ReadDaily(d)
		if strings.TrimSpace(content) == "" {
			continue
		}
		sb.WriteString(fmt.Sprintf("===== %s =====\n%s\n\n", d, content))
	}
	return sb.String()
}

// ---------- 知识库 ----------

func topicFile(topic string) string {
	topic = strings.TrimSpace(topic)
	topic = strings.ReplaceAll(topic, "/", "_")
	topic = strings.ReplaceAll(topic, "\\", "_")
	topic = strings.ReplaceAll(topic, "..", "_")
	return topic + ".md"
}

// WriteKnowledge 写入/覆盖一个知识主题文件。
func (s *Store) WriteKnowledge(topic, content string) error {
	if strings.TrimSpace(topic) == "" {
		return fmt.Errorf("topic 不能为空")
	}
	path := filepath.Join(s.rootDir, "knowledge", topicFile(topic))
	return os.WriteFile(path, []byte(content), 0644)
}

// ReadKnowledge 读取知识主题内容；topic 为空时返回索引列表。
func (s *Store) ReadKnowledge(topic string) (string, error) {
	if strings.TrimSpace(topic) == "" {
		entries, _ := os.ReadDir(filepath.Join(s.rootDir, "knowledge"))
		var names []string
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
				names = append(names, strings.TrimSuffix(e.Name(), ".md"))
			}
		}
		sort.Strings(names)
		return strings.Join(names, "\n"), nil
	}
	data, err := os.ReadFile(filepath.Join(s.rootDir, "knowledge", topicFile(topic)))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ---------- 提示词文件 ----------

var promptFiles = map[string]string{
	"agent": "AGENT.md",
	"user":  "USER.md",
	"rule":  "RULE.md",
}

// UpdatePrompt 更新提示词文件（agent/user/rule）。
func (s *Store) UpdatePrompt(name, content string) error {
	file, ok := promptFiles[strings.ToLower(name)]
	if !ok {
		return fmt.Errorf("未知提示词类型: %s（可选 agent/user/rule）", name)
	}
	return os.WriteFile(filepath.Join(s.rootDir, file), []byte(content), 0644)
}

// ReadPrompt 读取提示词文件内容。
func (s *Store) ReadPrompt(name string) string {
	file, ok := promptFiles[strings.ToLower(name)]
	if !ok {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(s.rootDir, file))
	if err != nil {
		return ""
	}
	return string(data)
}
