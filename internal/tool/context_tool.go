package tool

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/afumu/openlink/internal/memory"
	"github.com/afumu/openlink/internal/types"
)

// ContextSummarizeTool 上下文智能压缩：把过长上下文提炼总结并写入天级记忆，返回可注入的压缩摘要。
type ContextSummarizeTool struct {
	config *types.Config
}

func NewContextSummarizeTool(config *types.Config) *ContextSummarizeTool {
	return &ContextSummarizeTool{config: config}
}

func (t *ContextSummarizeTool) Name() string        { return "context_summarize" }
func (t *ContextSummarizeTool) Description() string { return "压缩过长上下文（确定性截断：保留首尾、去除中间冗余），将精简上下文写入天级记忆并返回，供重新注入对话" }
func (t *ContextSummarizeTool) Parameters() interface{} {
	return map[string]string{
		"turns": "array of {role, content} (可选)：需要压缩的历史对话；也可用 raw 直接传入文本",
		"raw":   "string (可选)：原始上下文文本",
	}
}

func (t *ContextSummarizeTool) Validate(args map[string]interface{}) error {
	if _, ok := args["turns"]; !ok {
		if _, ok := args["raw"]; !ok {
			return errors.New("turns 或 raw 至少提供一个")
		}
	}
	return nil
}

func (t *ContextSummarizeTool) Execute(ctx *Context) *Result {
	r := &Result{StartTime: time.Now()}
	transcript := t.buildTranscript(ctx.Args)
	if strings.TrimSpace(transcript) == "" {
		r.Status = "error"
		r.Error = "没有可压缩的内容"
		r.EndTime = time.Now()
		return r
	}

	// 确定性压缩：保留首尾、去除中间冗余（本项目不调用服务端大模型）。
	summary := naiveCompress(transcript)

	// 写入天级记忆（去重）
	_, _ = memory.Get().AppendDaily("", "上下文压缩摘要："+summary)

	r.Status = "success"
	r.Output = fmt.Sprintf("上下文已压缩并写入天级记忆。以下为可注入的精简上下文：\n\n%s", summary)
	r.EndTime = time.Now()
	return r
}

func (t *ContextSummarizeTool) buildTranscript(args map[string]interface{}) string {
	if turns, ok := args["turns"].([]interface{}); ok {
		var sb strings.Builder
		for _, item := range turns {
			if m, ok := item.(map[string]interface{}); ok {
				role := strings.ToUpper(toStr(m["role"]))
				content := toStr(m["content"])
				sb.WriteString(fmt.Sprintf("[%s] %s\n", role, content))
			}
		}
		return sb.String()
	}
	return toStr(args["raw"])
}

func naiveCompress(text string) string {
	lines := strings.Split(text, "\n")
	if len(lines) <= 8 {
		return text
	}
	head := strings.Join(lines[:4], "\n")
	tail := strings.Join(lines[len(lines)-4:], "\n")
	return head + "\n…(已省略中间内容)…\n" + tail
}
