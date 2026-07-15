package tool

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/afumu/openlink/internal/config"
	"github.com/afumu/openlink/internal/evolution"
	"github.com/afumu/openlink/internal/types"
)

// EvolutionControlTool 自进化控制：开关、手动触发复盘/梦境、查询状态。
// 说明：本项目的 LLM 运行在网页端，因此复盘/梦境的「语义推理」由网页 AI 完成：
// review_now/dream_now 返回供 LLM 推理的汇总材料(brief)；
// review_done/dream_done 收尾并产出变更日志。
type EvolutionControlTool struct {
	config *types.Config
}

func NewEvolutionControlTool(config *types.Config) *EvolutionControlTool {
	return &EvolutionControlTool{config: config}
}

func (t *EvolutionControlTool) Name() string        { return "evolution_control" }
func (t *EvolutionControlTool) Description() string { return "自进化控制：enable/disable 开关；review_now/dream_now 获取材料并自行复盘/蒸馏；review_done/dream_done 收尾；status 查询" }
func (t *EvolutionControlTool) Parameters() interface{} {
	return map[string]string{
		"action":  "enable | disable | review_now | review_done | dream_now | dream_done | status (required)",
		"days":    "（预留）梦境整理天数",
		"summary": "review_done / dream_done 时的复盘/整理总结文本",
	}
}

func (t *EvolutionControlTool) Validate(args map[string]interface{}) error {
	v, ok := args["action"].(string)
	if !ok {
		return fmt.Errorf("action 是必填项")
	}
	switch v {
	case "enable", "disable", "review_now", "review_done", "dream_now", "dream_done", "status":
		return nil
	}
	return fmt.Errorf("未知 action: %s", v)
}

func (t *EvolutionControlTool) Execute(ctx *Context) *Result {
	r := &Result{StartTime: time.Now()}
	action := ctx.Args["action"].(string)
	switch action {
	case "enable":
		config.Update(&config.Settings{Evolution: config.EvolutionSettings{Enabled: true}})
		r.Status = "success"
		r.Output = "自进化已开启"
	case "disable":
		config.Update(&config.Settings{Evolution: config.EvolutionSettings{Enabled: false}})
		r.Status = "success"
		r.Output = "自进化已关闭"
	case "status":
		st := evolution.Get().Status()
		data, _ := json.MarshalIndent(st, "", "  ")
		r.Status = "success"
		r.Output = string(data)
	case "review_now":
		out := evolution.Get().BeginReview()
		data, _ := json.MarshalIndent(out, "", "  ")
		r.Status = "success"
		r.Output = string(data)
	case "review_done":
		summary := toStr(ctx.Args["summary"])
		out := evolution.Get().FinishReview(summary)
		data, _ := json.MarshalIndent(out, "", "  ")
		r.Status = "success"
		r.Output = string(data)
	case "dream_now":
		out := evolution.Get().BeginDream()
		data, _ := json.MarshalIndent(out, "", "  ")
		r.Status = "success"
		r.Output = string(data)
	case "dream_done":
		summary := toStr(ctx.Args["summary"])
		out := evolution.Get().FinishDream(summary)
		data, _ := json.MarshalIndent(out, "", "  ")
		r.Status = "success"
		r.Output = string(data)
	}
	r.EndTime = time.Now()
	return r
}
