package tool

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/afumu/openlink/internal/memory"
	"github.com/afumu/openlink/internal/types"
)

// PromptUpdateTool 更新提示词文件（AGENT.md / USER.md / RULE.md）。
type PromptUpdateTool struct {
	config *types.Config
}

func NewPromptUpdateTool(config *types.Config) *PromptUpdateTool {
	return &PromptUpdateTool{config: config}
}

func (t *PromptUpdateTool) Name() string        { return "prompt_update" }
func (t *PromptUpdateTool) Description() string { return "更新 Agent 设定文件：file=agent|user|rule，写入对应 AGENT.md/USER.md/RULE.md" }
func (t *PromptUpdateTool) Parameters() interface{} {
	return map[string]string{
		"file":    "agent | user | rule (required)",
		"content": "完整文件内容 (required)",
	}
}

func (t *PromptUpdateTool) Validate(args map[string]interface{}) error {
	if v, ok := args["file"].(string); !ok || (v != "agent" && v != "user" && v != "rule") {
		return errors.New("file 必须是 agent / user / rule")
	}
	if _, ok := args["content"]; !ok {
		return errors.New("content 是必填项")
	}
	return nil
}

func (t *PromptUpdateTool) Execute(ctx *Context) *Result {
	r := &Result{StartTime: time.Now()}
	file := ctx.Args["file"].(string)
	content := toStr(ctx.Args["content"])
	var rel string
	switch file {
	case "agent":
		rel = "AGENT.md"
	case "user":
		rel = "USER.md"
	default:
		rel = "RULE.md"
	}
	path := filepath.Join(ctx.Config.RootDir, rel)
	t.config.Review.Snapshot(path)
	if err := memory.Get().UpdatePrompt(file, content); err != nil {
		r.Status = "error"
		r.Error = err.Error()
		r.EndTime = time.Now()
		return r
	}
	t.config.Review.RecordChange(path)
	r.Status = "success"
	r.Output = fmt.Sprintf("已更新 %s", rel)
	r.EndTime = time.Now()
	return r
}
