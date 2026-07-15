package tool

import (
	"errors"
	"time"

	"github.com/afumu/openlink/internal/session"
	"github.com/afumu/openlink/internal/types"
)

// SessionLogTool 将会话轮次记录到服务端，用于空闲检测与复盘。
type SessionLogTool struct {
	config *types.Config
}

func NewSessionLogTool(config *types.Config) *SessionLogTool {
	return &SessionLogTool{config: config}
}

func (t *SessionLogTool) Name() string        { return "session_log" }
func (t *SessionLogTool) Description() string { return "记录一次对话轮次（session_id/role/content），供自进化引擎做空闲检测与会话复盘" }
func (t *SessionLogTool) Parameters() interface{} {
	return map[string]string{
		"session_id": "会话ID (可选，默认 default)",
		"role":       "user | assistant | tool (required)",
		"content":    "该轮次文本内容 (required)",
	}
}

func (t *SessionLogTool) Validate(args map[string]interface{}) error {
	if v, ok := args["role"].(string); !ok || (v != "user" && v != "assistant" && v != "tool") {
		return errors.New("role 必须是 user / assistant / tool")
	}
	if _, ok := args["content"]; !ok {
		return errors.New("content 是必填项")
	}
	return nil
}

func (t *SessionLogTool) Execute(ctx *Context) *Result {
	r := &Result{StartTime: time.Now()}
	sid := toStr(ctx.Args["session_id"])
	role := toStr(ctx.Args["role"])
	content := toStr(ctx.Args["content"])
	session.Get().LogTurn(sid, role, content)
	r.Status = "success"
	r.Output = "已记录轮次"
	r.EndTime = time.Now()
	return r
}
