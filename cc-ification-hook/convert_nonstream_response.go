package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func handleNonStreamingResponse(w http.ResponseWriter, resp *http.Response, originalModel string) {
	addLog("[NonStream] Processing non-streaming response")

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		writeNonStreamError(w, err)
		return
	}

	var openaiResp OpenAINonStreamResponse
	if err := json.Unmarshal(body, &openaiResp); err != nil {
		writeNonStreamError(w, err)
		return
	}

	anthropicResp := convertOpenAIToAnthropicResponse(&openaiResp, originalModel)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(anthropicResp)
}

func convertOpenAIToAnthropicResponse(openaiResp *OpenAINonStreamResponse, originalModel string) *AnthropicResponse {
	messageID := fmt.Sprintf("msg_%d", time.Now().UnixNano())

	var content []any
	var stopReason string

	if len(openaiResp.Choices) > 0 {
		choice := openaiResp.Choices[0]

		reasoning := extractNonStreamReasoning(&choice.Message)
		if reasoning != "" {
			content = append(content, map[string]any{
				"type":     "thinking",
				"thinking": reasoning,
			})
		}

		if choice.Message.Content != "" {
			content = append(content, map[string]any{
				"type": "text",
				"text": choice.Message.Content,
			})
		}

		for _, tc := range choice.Message.ToolCalls {
			var inputData any
			json.Unmarshal([]byte(tc.Function.Arguments), &inputData)
			if inputData == nil {
				inputData = map[string]any{}
			}
			content = append(content, map[string]any{
				"type":  "tool_use",
				"id":    tc.ID,
				"name":  tc.Function.Name,
				"input": inputData,
			})
		}

		stopReason = convertFinishReason(choice.FinishReason)
	}

	if len(content) == 0 {
		content = append(content, map[string]any{
			"type": "text",
			"text": "",
		})
	}

	inputTokens := 0
	outputTokens := 0
	cachedTokens := 0
	responseTotalTokens := 0
	if openaiResp.Usage != nil {
		inputTokens = int(float64(openaiResp.Usage.PromptTokens) * tokenScaleFactor)
		outputTokens = int(float64(openaiResp.Usage.CompletionTokens) * tokenScaleFactor)
		responseTotalTokens = int(float64(openaiResp.Usage.TotalTokens) * tokenScaleFactor)
		if openaiResp.Usage.PromptTokensDetails != nil {
			cachedTokens = int(float64(openaiResp.Usage.PromptTokensDetails.CachedTokens) * tokenScaleFactor)
		}
	}

	statsMu.Lock()
	totalPromptTokens += int64(inputTokens)
	totalCompletionTokens += int64(outputTokens)
	totalCachedTokens += int64(cachedTokens)
	totalTokens += int64(responseTotalTokens)
	statsMu.Unlock()

	return &AnthropicResponse{
		ID:           messageID,
		Type:         "message",
		Role:         "assistant",
		Content:      content,
		Model:        originalModel,
		StopReason:   stopReason,
		StopSequence: nil,
		Usage: &AnthropicUsage{
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
		},
	}
}

func extractNonStreamReasoning(msg *OpenAINonStreamMessage) string {
	if msg.Reasoning != "" {
		return msg.Reasoning
	}
	if msg.ReasoningContent != "" {
		return msg.ReasoningContent
	}
	if len(msg.ReasoningDetails) > 0 {
		var parts []string
		for _, detail := range msg.ReasoningDetails {
			if detail.Content != "" {
				parts = append(parts, detail.Content)
			}
		}
		return strings.Join(parts, "")
	}
	return ""
}

func writeNonStreamError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{
			"type":    "api_error",
			"message": err.Error(),
		},
	})
}
