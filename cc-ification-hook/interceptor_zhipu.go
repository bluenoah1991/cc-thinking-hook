package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var (
	toolCallPattern = regexp.MustCompile(`<tool_call>([\s\S]*?)</tool_call>`)
	argKeyPattern   = regexp.MustCompile(`<arg_key>([\s\S]*?)</arg_key>`)
	argValuePattern = regexp.MustCompile(`<arg_value>([\s\S]*?)</arg_value>`)
)

type ZhipuInterceptorFactory struct{}

func (f *ZhipuInterceptorFactory) ShouldIntercept(backendURL string) bool {
	return strings.Contains(backendURL, "open.bigmodel.cn")
}

func (f *ZhipuInterceptorFactory) Create() Interceptor {
	return &ZhipuInterceptor{}
}

type ZhipuInterceptor struct {
	buffer string
}

func (z *ZhipuInterceptor) OnMessage(msg *AnthropicMessage) {
	if msg.Role != "assistant" {
		return
	}

	content, ok := msg.Content.([]any)
	if !ok {
		return
	}

	for _, block := range content {
		blockMap, ok := block.(map[string]any)
		if !ok {
			continue
		}

		if blockMap["type"] != "thinking" {
			continue
		}

		thinking, ok := blockMap["thinking"].(string)
		if !ok {
			continue
		}

		blockMap["thinking"] = toolCallPattern.ReplaceAllString(thinking, "")
	}
}

func (z *ZhipuInterceptor) OnDeltaStart(delta *OpenAIDelta) {
	if delta.ReasoningContent == "" {
		return
	}

	z.buffer += delta.ReasoningContent

	for {
		match := toolCallPattern.FindStringSubmatchIndex(z.buffer)
		if match == nil {
			break
		}

		tc := parseToolCall(z.buffer[match[2]:match[3]])
		if tc != nil {
			delta.ToolCalls = append(delta.ToolCalls, *tc)
			addLog(fmt.Sprintf("[Zhipu] Parsed tool_call: %s(%s)", tc.Function.Name, tc.Function.Arguments))
		}

		z.buffer = z.buffer[match[1]:]
	}
}

func parseToolCall(content string) *OpenAIToolCall {
	lines := strings.Split(strings.TrimSpace(content), "\n")
	if len(lines) == 0 {
		return nil
	}

	toolName := strings.TrimSpace(lines[0])
	if toolName == "" {
		return nil
	}

	args := parseArguments(lines[1:])

	return &OpenAIToolCall{
		ID:   fmt.Sprintf("call_%d", time.Now().UnixNano()),
		Type: "function",
		Function: ToolCallFunction{
			Name:      toolName,
			Arguments: args,
		},
	}
}

func parseArguments(lines []string) string {
	args := make(map[string]any)
	var currentKey string

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if match := argKeyPattern.FindStringSubmatch(line); match != nil {
			currentKey = match[1]
		} else if match := argValuePattern.FindStringSubmatch(line); match != nil && currentKey != "" {
			value := match[1]

			var parsed any
			if err := json.Unmarshal([]byte(value), &parsed); err == nil {
				args[currentKey] = parsed
			} else {
				args[currentKey] = value
			}

			currentKey = ""
		}
	}

	result, _ := json.Marshal(args)
	return string(result)
}

func init() {
	RegisterInterceptorFactory(&ZhipuInterceptorFactory{})
}
