package tool

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/afumu/openlink/internal/memory"
	"github.com/afumu/openlink/internal/types"
)

// KnowledgeWriteTool 写入知识库主题文件。
type KnowledgeWriteTool struct {
	config *types.Config
}

func NewKnowledgeWriteTool(config *types.Config) *KnowledgeWriteTool {
	return &KnowledgeWriteTool{config: config}
}

func (t *KnowledgeWriteTool) Name() string        { return "knowledge_write" }
func (t *KnowledgeWriteTool) Description() string { return "写入知识库：以 topic 为文件名保存 Markdown 知识（支持页面间交叉引用）" }
func (t *KnowledgeWriteTool) Parameters() interface{} {
	return map[string]string{
		"topic":   "知识主题名 (required)，作为 knowledge/<topic>.md",
		"content": "Markdown 内容 (required)",
	}
}

func (t *KnowledgeWriteTool) Validate(args map[string]interface{}) error {
	if _, ok := args["topic"]; !ok {
		return errors.New("topic 是必填项")
	}
	if _, ok := args["content"]; !ok {
		return errors.New("content 是必填项")
	}
	return nil
}

func (t *KnowledgeWriteTool) Execute(ctx *Context) *Result {
	r := &Result{StartTime: time.Now()}
	topic := toStr(ctx.Args["topic"])
	content := toStr(ctx.Args["content"])
	rel := filepath.Join("knowledge", sanitizeTopic(topic)+".md")
	path := filepath.Join(ctx.Config.RootDir, rel)
	t.config.Review.Snapshot(path)
	if err := memory.Get().WriteKnowledge(topic, content); err != nil {
		r.Status = "error"
		r.Error = err.Error()
		r.EndTime = time.Now()
		return r
	}
	t.config.Review.RecordChange(path)
	r.Status = "success"
	r.Output = fmt.Sprintf("已写入知识库：%s", rel)
	r.EndTime = time.Now()
	return r
}

// KnowledgeReadTool 读取知识库。
type KnowledgeReadTool struct {
	config *types.Config
}

func NewKnowledgeReadTool(config *types.Config) *KnowledgeReadTool {
	return &KnowledgeReadTool{config: config}
}

func (t *KnowledgeReadTool) Name() string        { return "knowledge_read" }
func (t *KnowledgeReadTool) Description() string { return "读取知识库：topic 为空时列出全部主题，否则返回该主题内容" }
func (t *KnowledgeReadTool) Parameters() interface{} {
	return map[string]string{
		"topic": "知识主题名 (可选)，为空则列出全部主题",
	}
}

func (t *KnowledgeReadTool) Validate(args map[string]interface{}) error { return nil }

func (t *KnowledgeReadTool) Execute(ctx *Context) *Result {
	r := &Result{StartTime: time.Now()}
	topic := toStr(ctx.Args["topic"])
	content, err := memory.Get().ReadKnowledge(topic)
	if err != nil {
		r.Status = "error"
		r.Error = err.Error()
		r.EndTime = time.Now()
		return r
	}
	r.Status = "success"
	if topic == "" {
		r.Output = "知识库主题列表：\n" + content
	} else {
		r.Output = content
	}
	r.EndTime = time.Now()
	return r
}

func sanitizeTopic(s string) string {
	out := make([]rune, 0, len(s))
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' {
			out = append(out, c)
		} else {
			out = append(out, '_')
		}
	}
	return string(out)
}
