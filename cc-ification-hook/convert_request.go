package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

func convertRequest(req *AnthropicRequest) (*ConvertResult, error) {
	useMultimodal := currentRoundHasImage(req) && multimodalURL != ""

	if useMultimodal && multimodalAPIType == "anthropic" {
		return &ConvertResult{
			AnthropicRequest: preprocessAnthropicRequest(req, true),
			UseMultimodal:    true,
			IsAnthropic:      true,
		}, nil
	}

	openaiReq, err := convertAnthropicToOpenAI(req, useMultimodal)
	if err != nil {
		return nil, err
	}

	return &ConvertResult{
		OpenAIRequest: openaiReq,
		UseMultimodal: useMultimodal,
		IsAnthropic:   false,
	}, nil
}

func preprocessAnthropicRequest(req *AnthropicRequest, useMultimodal bool) *AnthropicRequest {
	preprocessedReq := *req

	if useMultimodal {
		preprocessedReq.Model = multimodalModel
		if multimodalMaxTokens > 0 && preprocessedReq.MaxTokens > multimodalMaxTokens {
			preprocessedReq.MaxTokens = multimodalMaxTokens
		}
		addLog("[Multimodal] Image in last message, using multimodal API")
	}

	rounds := keepRounds
	if useMultimodal {
		rounds = 1
	}

	startIdx := 0
	if useMultimodal && multimodalMaxRounds > 0 {
		startIdx = getTrimBoundary(req.Messages, multimodalMaxRounds)
	}

	compressBoundary := getCompressBoundary(req.Messages, rounds)
	lastIdx := len(req.Messages) - 1
	roundStart := getLastRoundStart(req)

	var stats CompressionStats
	var preprocessedMessages []AnthropicMessage
	for i := startIdx; i <= lastIdx; i++ {
		msg := req.Messages[i]
		compress := rounds > 0 && i < compressBoundary
		isInLastRound := i >= roundStart
		preprocessedMsg := preprocessAnthropicMessage(msg, compress, isInLastRound, useMultimodal, &stats)
		if preprocessedMsg != nil {
			preprocessedMessages = append(preprocessedMessages, *preprocessedMsg)
		}
	}
	preprocessedReq.Messages = preprocessedMessages

	if stats.ThinkingBlocks > 0 || stats.ToolCalls > 0 || stats.ToolResults > 0 {
		addLog(fmt.Sprintf("[Compress] %d thinking, %d tool_use, %d tool_result", stats.ThinkingBlocks, stats.ToolCalls, stats.ToolResults))
	}

	return &preprocessedReq
}

func preprocessAnthropicMessage(msg AnthropicMessage, compress bool, isInLastRound bool, useMultimodal bool, stats *CompressionStats) *AnthropicMessage {
	content, ok := msg.Content.([]any)
	if !ok {
		return &msg
	}

	var preprocessedContent []any
	for _, block := range content {
		blockMap, ok := block.(map[string]any)
		if !ok {
			continue
		}

		blockType, _ := blockMap["type"].(string)
		switch blockType {
		case "thinking":
			if compress {
				stats.ThinkingBlocks++
				continue
			}
			preprocessedContent = append(preprocessedContent, block)
		case "tool_use":
			if compress {
				stats.ToolCalls++
				preprocessedContent = append(preprocessedContent, map[string]any{
					"type":  "tool_use",
					"id":    blockMap["id"],
					"name":  blockMap["name"],
					"input": map[string]any{"compressed": true},
				})
			} else {
				preprocessedContent = append(preprocessedContent, block)
			}
		case "tool_result":
			if compress {
				stats.ToolResults++
				preprocessedContent = append(preprocessedContent, map[string]any{
					"type":        "tool_result",
					"tool_use_id": blockMap["tool_use_id"],
					"content":     "[compressed]",
				})
			} else {
				preprocessedContent = append(preprocessedContent, block)
			}
		case "image":
			if isInLastRound && useMultimodal {
				preprocessedContent = append(preprocessedContent, block)
			} else {
				preprocessedContent = append(preprocessedContent, map[string]any{
					"type": "text",
					"text": "[image]",
				})
			}
		default:
			preprocessedContent = append(preprocessedContent, block)
		}
	}

	return &AnthropicMessage{
		Role:    msg.Role,
		Content: preprocessedContent,
	}
}

func convertAnthropicToOpenAI(req *AnthropicRequest, useMultimodal bool) (*OpenAIRequest, error) {
	openaiReq := &OpenAIRequest{
		Model:     req.Model,
		MaxTokens: req.MaxTokens,
		Stream:    req.Stream,
	}

	if useMultimodal {
		openaiReq.Model = multimodalModel
		if multimodalMaxTokens > 0 && openaiReq.MaxTokens > multimodalMaxTokens {
			openaiReq.MaxTokens = multimodalMaxTokens
		}
		addLog("[Multimodal] Image in last message, using multimodal API")
	} else if backendModel != "" {
		openaiReq.Model = backendModel
	}

	if req.Temperature != nil {
		openaiReq.Temperature = req.Temperature
	}
	if req.TopP != nil {
		openaiReq.TopP = req.TopP
	}
	if len(req.StopSequences) > 0 {
		openaiReq.Stop = req.StopSequences
	}

	if req.Stream {
		openaiReq.StreamOptions = &StreamOptions{IncludeUsage: true}
	}

	if req.Thinking != nil && req.Thinking.BudgetTokens > 0 {
		openaiReq.ReasoningEffort = budgetToEffort(req.Thinking.BudgetTokens)
	}

	var stats CompressionStats
	messages, err := convertMessages(req, useMultimodal, &stats)
	if err != nil {
		return nil, err
	}
	openaiReq.Messages = messages

	if stats.ThinkingBlocks > 0 || stats.ToolCalls > 0 || stats.ToolResults > 0 {
		addLog(fmt.Sprintf("[Compress] %d thinking, %d tool_use, %d tool_result", stats.ThinkingBlocks, stats.ToolCalls, stats.ToolResults))
	}

	if len(req.Tools) > 0 {
		openaiReq.Tools = convertTools(req.Tools)
	}

	if req.ToolChoice != nil {
		openaiReq.ToolChoice = convertToolChoice(req.ToolChoice)
	}

	return openaiReq, nil
}

func convertMessages(req *AnthropicRequest, useMultimodal bool, stats *CompressionStats) ([]OpenAIMessage, error) {
	var messages []OpenAIMessage

	if req.System != nil {
		systemContent := extractSystemContent(req.System)
		if systemContent != "" {
			messages = append(messages, OpenAIMessage{
				Role:    "system",
				Content: systemContent,
			})
		}
	}

	injectUltrathink := shouldInjectUltrathink(req)
	rounds := keepRounds
	if useMultimodal {
		rounds = 1
	}

	startIdx := 0
	if useMultimodal && multimodalMaxRounds > 0 {
		startIdx = getTrimBoundary(req.Messages, multimodalMaxRounds)
	}

	compressBoundary := getCompressBoundary(req.Messages, rounds)
	lastIdx := len(req.Messages) - 1
	roundStart := getLastRoundStart(req)

	for i := startIdx; i <= lastIdx; i++ {
		msg := req.Messages[i]
		injectPrompt := injectUltrathink && i == lastIdx
		compress := rounds > 0 && i < compressBoundary
		isInLastRound := i >= roundStart
		converted, err := convertMessage(msg, injectPrompt, compress, isInLastRound, useMultimodal, stats)
		if err != nil {
			return nil, err
		}
		messages = append(messages, converted...)
	}

	return messages, nil
}

func getCompressBoundary(messages []AnthropicMessage, rounds int) int {
	if rounds <= 0 {
		return 0
	}
	count := 0
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" && !hasToolResult(messages[i]) {
			count++
			if count >= rounds {
				return i
			}
		}
	}
	return 0
}

func getTrimBoundary(messages []AnthropicMessage, maxRounds int) int {
	count := 0
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" && !hasToolResult(messages[i]) {
			count++
			if count > maxRounds {
				for j := i + 1; j < len(messages); j++ {
					if messages[j].Role == "user" {
						return j
					}
				}
				return i + 1
			}
		}
	}
	return 0
}

func getLastRoundStart(req *AnthropicRequest) int {
	if len(req.Messages) == 0 {
		return 0
	}
	roundStart := 0
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" && !hasToolResult(req.Messages[i]) {
			roundStart = i
			break
		}
	}
	return roundStart
}

func currentRoundHasImage(req *AnthropicRequest) bool {
	if len(req.Messages) == 0 {
		return false
	}

	roundStart := getLastRoundStart(req)
	for i := roundStart; i < len(req.Messages); i++ {
		if messageHasImage(req.Messages[i]) {
			return true
		}
	}
	return false
}

func messageHasImage(msg AnthropicMessage) bool {
	content, ok := msg.Content.([]any)
	if !ok {
		return false
	}
	for _, block := range content {
		if blockMap, ok := block.(map[string]any); ok {
			if blockMap["type"] == "image" {
				return true
			}
		}
	}
	return false
}

func hasToolResult(msg AnthropicMessage) bool {
	content, ok := msg.Content.([]any)
	if !ok {
		return false
	}
	for _, block := range content {
		if blockMap, ok := block.(map[string]any); ok {
			if blockMap["type"] == "tool_result" {
				return true
			}
		}
	}
	return false
}

func convertMessage(msg AnthropicMessage, injectPrompt bool, compress bool, isInLastRound bool, useMultimodal bool, stats *CompressionStats) ([]OpenAIMessage, error) {
	if interceptor != nil {
		interceptor.OnMessage(&msg)
	}

	switch msg.Role {
	case "user":
		return convertUserMessage(msg, injectPrompt, compress, isInLastRound, useMultimodal, stats)
	case "assistant":
		return convertAssistantMessage(msg, compress, stats)
	}
	return nil, nil
}

func convertUserMessage(msg AnthropicMessage, injectPrompt bool, compress bool, isInLastRound bool, useMultimodal bool, stats *CompressionStats) ([]OpenAIMessage, error) {
	var messages []OpenAIMessage

	content, ok := msg.Content.([]any)
	if !ok {
		if str, ok := msg.Content.(string); ok {
			finalContent := str
			if injectPrompt {
				finalContent = str + "\n\n" + ultrathinkPrompt
				printInjectionLog(str)
			}
			messages = append(messages, OpenAIMessage{
				Role:    "user",
				Content: finalContent,
			})
		}
		return messages, nil
	}

	var contentParts []OpenAIContentPart
	var toolResults []OpenAIMessage
	seenToolResults := make(map[string]bool)

	for _, block := range content {
		blockMap, ok := block.(map[string]any)
		if !ok {
			continue
		}

		blockType, _ := blockMap["type"].(string)

		switch blockType {
		case "text":
			text, _ := blockMap["text"].(string)
			contentParts = append(contentParts, OpenAIContentPart{
				Type: "text",
				Text: text,
			})
		case "image":
			if isInLastRound && useMultimodal {
				source, ok := blockMap["source"].(map[string]any)
				if ok {
					mediaType, _ := source["media_type"].(string)
					data, _ := source["data"].(string)
					contentParts = append(contentParts, OpenAIContentPart{
						Type: "image_url",
						ImageURL: &ImageURL{
							URL: fmt.Sprintf("data:%s;base64,%s", mediaType, data),
						},
					})
				}
			} else {
				contentParts = append(contentParts, OpenAIContentPart{
					Type: "text",
					Text: "[image]",
				})
			}
		case "tool_result":
			toolUseID, _ := blockMap["tool_use_id"].(string)
			if seenToolResults[toolUseID] {
				continue
			}
			seenToolResults[toolUseID] = true
			resultContent := "[compressed]"
			if compress {
				stats.ToolResults++
			} else {
				resultContent = extractToolResultContent(blockMap["content"])
			}
			toolResults = append(toolResults, OpenAIMessage{
				Role:       "tool",
				Content:    resultContent,
				ToolCallID: toolUseID,
			})
		}
	}

	if len(toolResults) > 0 {
		messages = append(messages, toolResults...)
	}

	if len(contentParts) > 0 {
		if injectPrompt {
			var userText string
			for _, part := range contentParts {
				if part.Type == "text" && part.Text != "" {
					userText = part.Text
					break
				}
			}
			contentParts = append(contentParts, OpenAIContentPart{
				Type: "text",
				Text: ultrathinkPrompt,
			})
			printInjectionLog(userText)
		}
		messages = append(messages, OpenAIMessage{
			Role:    "user",
			Content: contentPartsToAny(contentParts),
		})
	}

	return messages, nil
}

func convertAssistantMessage(msg AnthropicMessage, compress bool, stats *CompressionStats) ([]OpenAIMessage, error) {
	var messages []OpenAIMessage

	content, ok := msg.Content.([]any)
	if !ok {
		if str, ok := msg.Content.(string); ok {
			messages = append(messages, OpenAIMessage{
				Role:    "assistant",
				Content: str,
			})
		}
		return messages, nil
	}

	var textParts []string
	var thinkingParts []string
	var toolCalls []OpenAIToolCall
	seenToolUse := make(map[string]bool)

	for _, block := range content {
		blockMap, ok := block.(map[string]any)
		if !ok {
			continue
		}

		blockType, _ := blockMap["type"].(string)

		switch blockType {
		case "thinking":
			if compress {
				stats.ThinkingBlocks++
			} else {
				thinking, _ := blockMap["thinking"].(string)
				thinkingParts = append(thinkingParts, thinking)
			}
		case "text":
			text, _ := blockMap["text"].(string)
			textParts = append(textParts, text)
		case "tool_use":
			id, _ := blockMap["id"].(string)
			if seenToolUse[id] {
				continue
			}
			seenToolUse[id] = true
			name, _ := blockMap["name"].(string)
			args := `{"compressed":true}`
			if compress {
				stats.ToolCalls++
			} else {
				input := blockMap["input"]
				inputJSON, _ := json.Marshal(input)
				args = string(inputJSON)
			}
			toolCalls = append(toolCalls, OpenAIToolCall{
				ID:   id,
				Type: "function",
				Function: ToolCallFunction{
					Name:      name,
					Arguments: args,
				},
			})
		}
	}

	assistantMsg := OpenAIMessage{Role: "assistant"}

	if len(thinkingParts) > 0 {
		assistantMsg.ReasoningContent = strings.Join(thinkingParts, "\n")
	}
	if len(textParts) > 0 {
		assistantMsg.Content = strings.Join(textParts, "\n")
	}
	if len(toolCalls) > 0 {
		assistantMsg.ToolCalls = toolCalls
	}

	contentStr, _ := assistantMsg.Content.(string)
	if contentStr != "" || assistantMsg.ReasoningContent != "" || len(assistantMsg.ToolCalls) > 0 {
		messages = append(messages, assistantMsg)
	}

	return messages, nil
}

func convertTools(tools []AnthropicTool) []OpenAITool {
	var result []OpenAITool
	for _, tool := range tools {
		result = append(result, OpenAITool{
			Type: "function",
			Function: ToolFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  removeUriFormat(tool.InputSchema),
			},
		})
	}
	return result
}

func convertToolChoice(tc *AnthropicToolChoice) any {
	switch tc.Type {
	case "none":
		return "none"
	case "auto":
		return "auto"
	case "any":
		return "required"
	case "tool":
		return map[string]any{
			"type": "function",
			"function": map[string]any{
				"name": tc.Name,
			},
		}
	}
	return "auto"
}

func shouldInjectUltrathink(req *AnthropicRequest) bool {
	if ultrathinkPrompt == "" {
		return false
	}
	if req.Thinking == nil || req.Thinking.BudgetTokens == 0 {
		return false
	}
	if len(req.Messages) == 0 {
		return false
	}
	return req.Messages[len(req.Messages)-1].Role == "user"
}

func budgetToEffort(budgetTokens int) string {
	if budgetTokens < 4000 {
		return "low"
	}
	if budgetTokens >= 32000 {
		return "high"
	}
	return "medium"
}

func printInjectionLog(userContent string) {
	preview := userContent
	if len(preview) > 20 {
		preview = preview[:20] + "..."
	}
	addLog(fmt.Sprintf("[✓] Injected prompt: %s", preview))
}

func extractSystemContent(system any) string {
	switch v := system.(type) {
	case string:
		return v
	case []any:
		var parts []string
		for _, item := range v {
			switch t := item.(type) {
			case string:
				parts = append(parts, t)
			case map[string]any:
				if text, ok := t["text"].(string); ok {
					parts = append(parts, text)
				}
			}
		}
		return strings.Join(parts, "\n\n")
	}
	return ""
}

func extractToolResultContent(content any) string {
	switch v := content.(type) {
	case string:
		return v
	case []any:
		var parts []string
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				if text, ok := m["text"].(string); ok {
					parts = append(parts, text)
				}
			}
		}
		return strings.Join(parts, "\n")
	default:
		data, _ := json.Marshal(v)
		return string(data)
	}
}

func contentPartsToAny(parts []OpenAIContentPart) any {
	if len(parts) == 1 && parts[0].Type == "text" {
		return parts[0].Text
	}
	result := make([]any, len(parts))
	for i, p := range parts {
		result[i] = p
	}
	return result
}

func removeUriFormat(schema any) any {
	if schema == nil {
		return nil
	}

	switch v := schema.(type) {
	case map[string]any:
		result := make(map[string]any)
		for key, val := range v {
			if key == "format" && val == "uri" {
				if t, ok := v["type"]; ok && t == "string" {
					continue
				}
			}
			if key == "properties" {
				if props, ok := val.(map[string]any); ok {
					newProps := make(map[string]any)
					for propKey, propVal := range props {
						newProps[propKey] = removeUriFormat(propVal)
					}
					result[key] = newProps
					continue
				}
			}
			if key == "items" || key == "additionalProperties" {
				result[key] = removeUriFormat(val)
				continue
			}
			if key == "anyOf" || key == "allOf" || key == "oneOf" {
				if arr, ok := val.([]any); ok {
					newArr := make([]any, len(arr))
					for i, item := range arr {
						newArr[i] = removeUriFormat(item)
					}
					result[key] = newArr
					continue
				}
			}
			result[key] = removeUriFormat(val)
		}
		return result
	case []any:
		result := make([]any, len(v))
		for i, item := range v {
			result[i] = removeUriFormat(item)
		}
		return result
	default:
		return v
	}
}
