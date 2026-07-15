package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// EvolutionSettings 自进化引擎配置。
// 注意：本项目的大模型运行在网页端（Gemini/DeepSeek 等），
// 本地 Go 服务不调用任何大模型，因此无需 LLM 配置。
// 会话复盘与梦境整理的「语义推理」由网页端 LLM 通过调用工具完成，
// 服务端只负责触发提醒、汇总材料(brief)、变更快照、变更日志与回滚。
type EvolutionSettings struct {
	Enabled     bool   `json:"enabled"`      // 自进化总开关
	IdleMinutes int    `json:"idle_minutes"` // 空闲多久触发复盘（默认 10）
	MinRounds   int    `json:"min_rounds"`   // 触发复盘所需最少会话轮次（默认 6）
	DreamTime   string `json:"dream_time"`   // 每日梦境整理时间，HH:MM（默认 23:55）
}

// Settings 运行时配置，保存在 .openlink/config.json。
type Settings struct {
	Evolution EvolutionSettings `json:"evolution"`
}

// Current 全局当前配置（服务启动时由 Load 初始化，控制台可热更新）。
var Current *Settings

// loadedPath 实际加载/保存的配置文件路径。
var loadedPath string

var mu sync.RWMutex

// homeConfigDir 返回 ~/.openlink 目录。
func homeConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".openlink")
}

// resolvePath 优先使用工作目录下的 .openlink/config.json，其次 ~/.openlink/config.json。
func resolvePath(rootDir string) string {
	local := filepath.Join(rootDir, ".openlink", "config.json")
	if _, err := os.Stat(local); err == nil {
		return local
	}
	return filepath.Join(homeConfigDir(), "config.json")
}

// defaults 返回带默认值的配置。
func defaults() *Settings {
	return &Settings{
		Evolution: EvolutionSettings{
			Enabled:     false,
			IdleMinutes: 10,
			MinRounds:   6,
			DreamTime:   "23:55",
		},
	}
}

// Load 加载配置，文件不存在则用默认值并写入。rootDir 用于定位工作目录级配置。
func Load(rootDir string) *Settings {
	s := defaults()
	path := resolvePath(rootDir)
	loadedPath = path
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, s)
	}
	if s.Evolution.MinRounds <= 0 {
		s.Evolution.MinRounds = 6
	}
	if s.Evolution.IdleMinutes <= 0 {
		s.Evolution.IdleMinutes = 10
	}
	if s.Evolution.DreamTime == "" {
		s.Evolution.DreamTime = "23:55"
	}
	mu.Lock()
	Current = s
	mu.Unlock()
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	_ = persist(s)
	return s
}

// persist 写回 loadedPath（不持锁，供已持有数据者调用）。
func persist(s *Settings) error {
	if loadedPath == "" {
		return nil
	}
	_ = os.MkdirAll(filepath.Dir(loadedPath), 0755)
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(loadedPath, data, 0644)
}

// Save 将当前配置写回磁盘。
func Save() error {
	mu.RLock()
	s := Current
	mu.RUnlock()
	if s == nil {
		return nil
	}
	return persist(s)
}

// Get 返回当前配置（线程安全）。
func Get() *Settings {
	mu.RLock()
	defer mu.RUnlock()
	return Current
}

// Update 浅合并更新并落盘。非零字段覆盖，Enabled 直接采用传入值。
func Update(patch *Settings) *Settings {
	mu.Lock()
	if Current == nil {
		Current = defaults()
	}
	Current.Evolution.Enabled = patch.Evolution.Enabled
	if patch.Evolution.IdleMinutes > 0 {
		Current.Evolution.IdleMinutes = patch.Evolution.IdleMinutes
	}
	if patch.Evolution.MinRounds > 0 {
		Current.Evolution.MinRounds = patch.Evolution.MinRounds
	}
	if patch.Evolution.DreamTime != "" {
		Current.Evolution.DreamTime = patch.Evolution.DreamTime
	}
	cp := *Current
	mu.Unlock()

	_ = persist(&cp)
	return &cp
}
