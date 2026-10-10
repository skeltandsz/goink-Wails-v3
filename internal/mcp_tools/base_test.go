package mcp_tools

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
)

type systemErrorTestArgs struct{}

type systemErrorTestTool struct {
	err    error
	result *ToolResult
}

func (t systemErrorTestTool) Name() string           { return "system_error_test" }
func (t systemErrorTestTool) Description() string    { return "test only" }
func (t systemErrorTestTool) Category() ToolCategory { return CategoryWritingAssistant }
func (t systemErrorTestTool) JSONSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object"}`)
}
func (t systemErrorTestTool) ExposeToLLM() bool { return false }
func (t systemErrorTestTool) NewArgs() any      { return &systemErrorTestArgs{} }
func (t systemErrorTestTool) Execute(context.Context, any, ToolContext) (*ToolResult, error) {
	return t.result, t.err
}

func TestRegistryExecute_CompactsToolResultError(t *testing.T) {
	reg := NewRegistry(slog.New(slog.NewTextHandler(io.Discard, nil)))
	detail := "文件已被修改，请重新读取\n" + strings.Repeat("x", maxSystemErrorDetailRunes+1)
	reg.Register(systemErrorTestTool{result: &ToolResult{Success: false, Error: detail}})

	result := reg.Execute(context.Background(), "system_error_test", []byte(`{}`), ToolContext{}, nil)
	if result.Success {
		t.Fatal("expected failure")
	}
	if result.ErrKind != ErrKindBusiness {
		t.Errorf("ErrKind = %q, want business kind", result.ErrKind)
	}
	if strings.Contains(result.Error, "\n") {
		t.Errorf("Error contains newline: %q", result.Error)
	}
	if !strings.HasPrefix(result.Error, "文件已被修改，请重新读取 x") {
		t.Errorf("Error = %q, want compacted ToolResult detail", result.Error)
	}
	if !strings.HasSuffix(result.Error, "…") {
		t.Errorf("Error = %q, want truncation marker", result.Error)
	}
}

func TestRegistryExecute_SystemErrorIncludesCompactDetail(t *testing.T) {
	reg := NewRegistry(slog.New(slog.NewTextHandler(io.Discard, nil)))
	detail := "正文已回退，标题未更新: database is locked\n" + strings.Repeat("x", maxSystemErrorDetailRunes+1)
	reg.Register(systemErrorTestTool{err: errors.New(detail)})

	result := reg.Execute(context.Background(), "system_error_test", []byte(`{}`), ToolContext{}, nil)
	if result.Success {
		t.Fatal("expected system error")
	}
	if result.ErrKind != ErrKindSystem {
		t.Errorf("ErrKind = %q, want %q", result.ErrKind, ErrKindSystem)
	}
	if !strings.HasPrefix(result.Error, "工具执行失败：正文已回退，标题未更新: database is locked x") {
		t.Errorf("Error = %q, want compact detail with operation state", result.Error)
	}
	if strings.Contains(result.Error, "\n") {
		t.Errorf("Error contains newline: %q", result.Error)
	}
	if !strings.HasSuffix(result.Error, "…") {
		t.Errorf("Error = %q, want truncation marker", result.Error)
	}
	if got := len([]rune(strings.TrimPrefix(result.Error, "工具执行失败："))); got != maxSystemErrorDetailRunes+1 {
		t.Errorf("detail rune length = %d, want %d", got, maxSystemErrorDetailRunes+1)
	}
}
