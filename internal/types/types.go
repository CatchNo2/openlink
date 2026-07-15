package types

import "encoding/json"

type ToolRequest struct {
	Name   string                 `json:"name"`
	Args   map[string]interface{} `json:"args"`
	Reason string                 `json:"reason,omitempty"`
}

func (r *ToolRequest) UnmarshalJSON(data []byte) error {
	type raw struct {
		Name      string                 `json:"name"`
		Args      map[string]interface{} `json:"args"`
		Arguments map[string]interface{} `json:"arguments"`
		Reason    string                 `json:"reason,omitempty"`
	}
	var v raw
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	r.Name = v.Name
	r.Reason = v.Reason
	if v.Args != nil {
		r.Args = v.Args
	} else {
		r.Args = v.Arguments
	}
	return nil
}

type ToolResponse struct {
	Status     string `json:"status"`
	Output     string `json:"output"`
	Error      string `json:"error,omitempty"`
	StopStream bool   `json:"stopStream,omitempty"`
	HasReview  bool   `json:"hasReview,omitempty"`
}

type ReviewManager interface {
	Snapshot(path string)
	RecordChange(path string)
	Review() []map[string]interface{}
	HasSession() bool
	Undo(path string) ([]string, error)
	Keep(paths []string) []string
}

type Config struct {
	RootDir       string
	Port          int
	Timeout       int
	Token         string
	DefaultPrompt []byte
	Review        ReviewManager
}

type Settings struct {
	Token     string `json:"token"`
	CreatedAt string `json:"created_at"`
}
