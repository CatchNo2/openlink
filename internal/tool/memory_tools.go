package tool

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/afumu/openlink/internal/memory"
	"github.com/afumu/openlink/internal/types"
)

// MemoryWriteTool 写入核心/天级记忆（带哈希去重与撤销快照）。
type MemoryWriteTool struct {
	config *types.Config
}

func NewMemoryWriteTool(config *types.Config) *MemoryWriteTool {
	return &MemoryWriteTool{config: config}
}

func (t *MemoryWriteTool) Name() string        { return "memory_write" }
func (t *MemoryWriteTool) Description() string { return "写入长期记忆：type=core 写入核心记忆(MEMORY.md)，type=daily 写入当天天级记忆(自动去重)" }
func (t *MemoryWriteTool) Parameters() interface{} {
	return map[string]string{
		"type":    "core | daily (required)",
		"content": "要写入的记忆内容 (required)",
		"date":    "daily 类型时的日期 YYYY-MM-DD，留空为今天",
	}
}

func (t *MemoryWriteTool) Validate(args map[string]interface{}) error {
	if v, ok := args["type"].(string); !ok || (v != "core" && v != "daily") {
		return errors.New("type 必须是 core 或 daily")
	}
	if _, ok := args["content"]; !ok {
		return errors.New("content 是必填项")
	}
	return nil
}

func (t *MemoryWriteTool) Execute(ctx *Context) *Result {
	r := &Result{StartTime: time.Now()}
	mtype := ctx.Args["type"].(string)
	content := toStr(ctx.Args["content"])
	date := toStr(ctx.Args["date"])

	var path string
	var written bool
	var err error
	if mtype == "core" {
		path = filepath.Join(ctx.Config.RootDir, "MEMORY.md")
		t.config.Review.Snapshot(path, "memory_write")
		written, err = memory.Get().AppendCore(content)
	} else {
		if date == "" {
			date = time.Now().Format("2006-01-02")
		}
		path = filepath.Join(ctx.Config.RootDir, "memory", date+".md")
		t.config.Review.Snapshot(path, "memory_write")
		written, err = memory.Get().AppendDaily(date, content)
	}
	if err != nil {
		r.Status = "error"
		r.Error = err.Error()
		r.EndTime = time.Now()
		return r
	}
	t.config.Review.RecordChange(path, "memory_write")
	if written {
		r.Status = "success"
		r.Output = fmt.Sprintf("已写入%s记忆。", zhType(mtype))
	} else {
		r.Status = "success"
		r.Output = "内容已存在（哈希去重，跳过写入）。"
	}
	r.EndTime = time.Now()
	return r
}

// MemoryReadTool 读取记忆。
type MemoryReadTool struct {
	config *types.Config
}

func NewMemoryReadTool(config *types.Config) *MemoryReadTool {
	return &MemoryReadTool{config: config}
}

func (t *MemoryReadTool) Name() string        { return "memory_read" }
func (t *MemoryReadTool) Description() string { return "读取记忆：type=core 读核心记忆，type=daily 读天级记忆" }
func (t *MemoryReadTool) Parameters() interface{} {
	return map[string]string{
		"type": "core | daily (required)",
		"date": "daily 类型时的日期，留空为今天",
	}
}

func (t *MemoryReadTool) Validate(args map[string]interface{}) error {
	if v, ok := args["type"].(string); !ok || (v != "core" && v != "daily") {
		return errors.New("type 必须是 core 或 daily")
	}
	return nil
}

func (t *MemoryReadTool) Execute(ctx *Context) *Result {
	r := &Result{StartTime: time.Now()}
	mtype := ctx.Args["type"].(string)
	date := toStr(ctx.Args["date"])
	var content string
	if mtype == "core" {
		content = memory.Get().ReadCore()
	} else {
		content = memory.Get().ReadDaily(date)
	}
	r.Status = "success"
	r.Output = content
	if r.Output == "" {
		r.Output = "(空)"
	}
	r.EndTime = time.Now()
	return r
}

func zhType(t string) string {
	if t == "core" {
		return "核心"
	}
	return "天级"
}

func toStr(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}
