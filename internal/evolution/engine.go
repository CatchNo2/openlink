package evolution

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/afumu/openlink/internal/config"
	"github.com/afumu/openlink/internal/memory"
	"github.com/afumu/openlink/internal/session"
	"github.com/afumu/openlink/internal/skill"
)

// Engine 自进化引擎：会话后复盘 + 梦境记忆整理。
//
// 重要架构约定：本项目的大模型运行在网页端（Gemini/DeepSeek 等），
// 本地 Go 服务只负责「工具执行 + 存储 + 编排」，不调用任何大模型。
// 因此复盘/梦境所需的「语义推理」由网页端 LLM 通过调用工具完成：
//   - 服务端在空闲/定时条件满足时设置「待办」并推送通知；
//   - 网页 AI 调用 evolution_control(review_now/dream_now) 获取汇总材料(brief)；
//   - 网页 AI 自行推理，并通过 memory/prompt/skill 等工具把结果落盘；
//   - 网页 AI 调用 evolution_control(review_done/dream_done) 收尾；
//   - 服务端比对变更快照，生成变更日志与通知（无变更则静默），并保留回滚备份。
type Engine struct {
	rootDir string
	mu      sync.Mutex

	pendingReview bool
	pendingDream  bool
	inReview      bool
	inDream       bool
	snapReview    map[string]string
	snapDream     map[string]string
	backReview    []string
	backDream     []string
}

var (
	eng     *Engine
	engOnce sync.Once
)

// Init 初始化引擎（仅一次）。
func Init(rootDir string) {
	engOnce.Do(func() {
		eng = &Engine{rootDir: rootDir}
		initBackup(rootDir)
		InitLog(rootDir)
	})
}

// Get 获取单例。
func Get() *Engine {
	if eng == nil {
		Init(".")
	}
	return eng
}

// Status 返回当前自进化状态摘要。
func (e *Engine) Status() map[string]interface{} {
	cfg := config.Get()
	e.mu.Lock()
	pendingReview, pendingDream, inReview, inDream := e.pendingReview, e.pendingDream, e.inReview, e.inDream
	e.mu.Unlock()
	sid := session.Get().MostActiveID()
	rounds := 0
	idleSec := 0
	if sid != "" {
		rounds = session.Get().RoundCount(sid)
		idleSec = int(time.Now().Unix() - session.Get().LastActivity(sid))
	}
	return map[string]interface{}{
		"enabled":        cfg.Evolution.Enabled,
		"idle_minutes":   cfg.Evolution.IdleMinutes,
		"min_rounds":     cfg.Evolution.MinRounds,
		"dream_time":     cfg.Evolution.DreamTime,
		"pending_review": pendingReview,
		"pending_dream":  pendingDream,
		"in_review":      inReview,
		"in_dream":       inDream,
		"active_session": sid,
		"rounds":         rounds,
		"idle_seconds":   idleSec,
	}
}

// MaybeReview 由空闲监控周期性调用：满足全部条件时请求一次复盘（提醒网页 AI 执行）。
func (e *Engine) MaybeReview() {
	cfg := config.Get()
	if cfg == nil || !cfg.Evolution.Enabled {
		return
	}
	sid := session.Get().MostActiveID()
	if sid == "" {
		return
	}
	rounds := session.Get().RoundCount(sid)
	if rounds < cfg.Evolution.MinRounds {
		return
	}
	idle := time.Now().Unix() - session.Get().LastActivity(sid)
	if idle < int64(cfg.Evolution.IdleMinutes*60) {
		return
	}
	e.mu.Lock()
	if e.pendingReview || e.inReview {
		e.mu.Unlock()
		return
	}
	e.mu.Unlock()
	e.RequestReview()
}

// RequestReview 设置「建议复盘」待办并推送通知（幂等）。
func (e *Engine) RequestReview() {
	e.mu.Lock()
	if e.pendingReview || e.inReview {
		e.mu.Unlock()
		return
	}
	e.pendingReview = true
	e.mu.Unlock()
	Log().AddNotification("建议进行会话复盘",
		"会话已空闲且达到轮次阈值。可在对话中调用 evolution_control(review_now) 获取会话材料并自行完成复盘。")
	log.Println("[Evolution] 已请求会话复盘（等待网页 AI 执行）")
}

// BeginReview 由网页 AI 通过 evolution_control(review_now) 调用：
// 建立变更快照（用于回滚与变更日志），返回供 LLM 推理的会话材料(brief)。
func (e *Engine) BeginReview() map[string]interface{} {
	e.mu.Lock()
	if e.inReview {
		e.mu.Unlock()
		return errMap("复盘已在进行中")
	}
	sid := session.Get().MostActiveID()
	if sid == "" {
		e.mu.Unlock()
		return errMap("没有可复盘的会话")
	}
	snap, backs := e.snapshotTracked()
	e.snapReview = snap
	e.backReview = backs
	e.inReview = true
	e.pendingReview = false
	e.mu.Unlock()

	topics, _ := memory.Get().ReadKnowledge("")
	brief := map[string]interface{}{
		"session_id":      sid,
		"transcript":     session.Get().Transcript(sid, 200),
		"skills":         skill.LoadInfos(e.rootDir),
		"core_memory":    memory.Get().ReadCore(),
		"knowledge_topics": topics,
		"instructions": "你是执行自进化复盘的 Agent。基于以上材料决定是否需要：\n" +
			"1) 把可复用流程固化成新技能（用 skill 工具写入 .skills/<name>/SKILL.md）；\n" +
			"2) 修复已有技能的问题（同样用 skill 工具覆盖写入）；\n" +
			"3) 把值得长期记住的偏好/决策/经验写入核心记忆（memory_write type=core）；\n" +
			"4) 把未完成任务记录到天级记忆（memory_write type=daily）。\n" +
			"完成后调用 evolution_control(review_done, summary=你的复盘总结)。若无需改动，也请调用 review_done 并说明。",
	}
	return brief
}

// FinishReview 由网页 AI 通过 evolution_control(review_done, summary=...) 调用：
// 比对变更快照，生成变更日志与通知（无变更则静默），并清理状态。
func (e *Engine) FinishReview(summary string) map[string]interface{} {
	e.mu.Lock()
	if !e.inReview {
		e.mu.Unlock()
		return errMap("当前没有进行中的复盘")
	}
	changes := e.diffTracked(e.snapReview)
	backs := e.backReview
	e.snapReview = nil
	e.backReview = nil
	e.inReview = false
	e.pendingReview = false
	e.mu.Unlock()

	if len(changes) > 0 || strings.TrimSpace(summary) != "" {
		Log().AddRecord(EvolutionRecord{
			Type:     "review",
			Summary:  summary,
			Changes:  changes,
			Rollback: backs,
		})
		if len(changes) > 0 {
			var sb strings.Builder
			sb.WriteString(summary + "\n")
			for _, c := range changes {
				sb.WriteString(fmt.Sprintf("- %s %s\n", c.Action, c.Path))
			}
			Log().AddNotification("会话复盘完成", sb.String())
		}
	}
	log.Printf("[Evolution] 复盘结束, 变更=%d\n", len(changes))
	return map[string]interface{}{"ok": true, "changes": changes, "summary": summary}
}

// RequestDream 设置「建议梦境整理」待办并推送通知（无新日记则跳过）。
func (e *Engine) RequestDream() {
	e.mu.Lock()
	if e.pendingDream || e.inDream {
		e.mu.Unlock()
		return
	}
	e.mu.Unlock()
	daily := memory.Get().CollectDaily(1)
	if strings.TrimSpace(daily) == "" {
		return
	}
	e.mu.Lock()
	e.pendingDream = true
	e.mu.Unlock()
	Log().AddNotification("建议进行梦境记忆整理",
		"当天有新的天级记忆。可在对话中调用 evolution_control(dream_now) 获取材料并自行完成记忆蒸馏。")
	log.Println("[Evolution] 已请求梦境整理（等待网页 AI 执行）")
}

// BeginDream 由网页 AI 通过 evolution_control(dream_now) 调用：建立快照并返回记忆材料。
func (e *Engine) BeginDream() map[string]interface{} {
	e.mu.Lock()
	if e.inDream {
		e.mu.Unlock()
		return errMap("梦境整理已在进行中")
	}
	snap, backs := e.snapshotTracked()
	e.snapDream = snap
	e.backDream = backs
	e.inDream = true
	e.pendingDream = false
	e.mu.Unlock()

	brief := map[string]interface{}{
		"current_core_memory": memory.Get().ReadCore(),
		"recent_daily_memory": memory.Get().CollectDaily(7),
		"instructions": "你是执行梦境记忆整理的 Agent。基于当前核心记忆与近期天级记忆，进行去重/合并/修剪，\n" +
			"输出精炼后的核心记忆（用 memory_write type=core 写回 MEMORY.md），并生成一篇叙事风格的梦境日记（用 memory_write type=daily 写入）。\n" +
			"核心记忆控制在约 50 条以内，保留标题结构。完成后调用 evolution_control(dream_done, summary=总结)。",
	}
	return brief
}

// FinishDream 由网页 AI 通过 evolution_control(dream_done, summary=...) 调用。
func (e *Engine) FinishDream(summary string) map[string]interface{} {
	e.mu.Lock()
	if !e.inDream {
		e.mu.Unlock()
		return errMap("当前没有进行中的梦境整理")
	}
	changes := e.diffTracked(e.snapDream)
	backs := e.backDream
	e.snapDream = nil
	e.backDream = nil
	e.inDream = false
	e.pendingDream = false
	e.mu.Unlock()

	if len(changes) > 0 || strings.TrimSpace(summary) != "" {
		Log().AddRecord(EvolutionRecord{
			Type:     "dream",
			Summary:  summary,
			Changes:  changes,
			Rollback: backs,
		})
		if len(changes) > 0 {
			var sb strings.Builder
			sb.WriteString(summary + "\n")
			for _, c := range changes {
				sb.WriteString(fmt.Sprintf("- %s %s\n", c.Action, c.Path))
			}
			Log().AddNotification("梦境记忆整理完成", sb.String())
		}
	}
	log.Printf("[Evolution] 梦境整理结束, 变更=%d\n", len(changes))
	return map[string]interface{}{"ok": true, "changes": changes, "summary": summary}
}

// ---------- 快照与差异 ----------

// trackedFiles 返回需要纳入自进化变更追踪的文件清单。
func (e *Engine) trackedFiles() []string {
	files := []string{"MEMORY.md", "AGENT.md", "USER.md", "RULE.md"}
	skillDir := filepath.Join(e.rootDir, ".skills")
	_ = filepath.Walk(skillDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if info.Name() == "SKILL.md" {
			if rel, rerr := filepath.Rel(e.rootDir, path); rerr == nil {
				files = append(files, rel)
			}
		}
		return nil
	})
	return files
}

// snapshotTracked 记录受追踪文件的当前内容（用于差异检测），并为每个文件创建回滚备份。
func (e *Engine) snapshotTracked() (map[string]string, []string) {
	m := map[string]string{}
	var backs []string
	for _, rel := range e.trackedFiles() {
		full := filepath.Join(e.rootDir, rel)
		data, err := os.ReadFile(full)
		if err != nil {
			m[rel] = ""
			continue
		}
		m[rel] = string(data)
		backs = append(backs, backupFile(e.rootDir, rel))
	}
	return m, backs
}

// diffTracked 比对当前受追踪文件与快照，返回变更条目。
func (e *Engine) diffTracked(orig map[string]string) []ChangeItem {
	cur := map[string]string{}
	for _, rel := range e.trackedFiles() {
		full := filepath.Join(e.rootDir, rel)
		data, err := os.ReadFile(full)
		if err != nil {
			cur[rel] = ""
			continue
		}
		cur[rel] = string(data)
	}
	all := map[string]bool{}
	for k := range orig {
		all[k] = true
	}
	for k := range cur {
		all[k] = true
	}
	var changes []ChangeItem
	for rel := range all {
		o := orig[rel]
		c := cur[rel]
		if o == c {
			continue
		}
		action := "modified"
		if o == "" {
			action = "created"
		} else if c == "" {
			action = "deleted"
		}
		changes = append(changes, ChangeItem{Action: action, Path: rel})
	}
	return changes
}

func errMap(msg string) map[string]interface{} {
	return map[string]interface{}{"ok": false, "error": msg}
}
