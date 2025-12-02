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
	argPairPattern  = regexp.MustCompile(`(?s)<arg_key>(.*?)</arg_key>\s*<arg_value>(.*?)</arg_value>`)
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
	lines := strings.SplitN(strings.TrimSpace(content), "\n", 2)
	if len(lines) == 0 {
		return nil
	}

	toolName := strings.TrimSpace(lines[0])
	if toolName == "" {
		return nil
	}

	var argsContent string
	if len(lines) > 1 {
		argsContent = lines[1]
	}

	args := parseArguments(argsContent)

	return &OpenAIToolCall{
		ID:   fmt.Sprintf("call_%d", time.Now().UnixNano()),
		Type: "function",
		Function: ToolCallFunction{
			Name:      toolName,
			Arguments: args,
		},
	}
}

func parseArguments(content string) string {
	args := make(map[string]any)

	matches := argPairPattern.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		if len(match) < 3 {
			continue
		}

		key := strings.TrimSpace(match[1])
		value := strings.TrimSpace(match[2])

		var parsed any
		if err := json.Unmarshal([]byte(value), &parsed); err == nil {
			args[key] = parsed
		} else {
			args[key] = value
		}
	}

	result, _ := json.Marshal(args)
	return string(result)
}

func init() {
	RegisterInterceptorFactory(&ZhipuInterceptorFactory{})
}
