package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

func handleNonStreamingResponse(w http.ResponseWriter, resp *http.Response, originalModel string) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		writeError(w, err)
		return
	}

	var openaiResp OpenAIResponse
	if err := json.Unmarshal(body, &openaiResp); err != nil {
		writeError(w, err)
		return
	}

	anthropicResp := convertOpenAIToAnthropic(&openaiResp, originalModel)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	json.NewEncoder(w).Encode(anthropicResp)
}

func convertOpenAIToAnthropic(openaiResp *OpenAIResponse, originalModel string) *AnthropicResponse {
	anthropicResp := &AnthropicResponse{
		ID:           openaiResp.ID,
		Type:         "message",
		Role:         "assistant",
		Model:        originalModel,
		StopSequence: nil,
		Content:      []AnthropicContentBlock{},
	}

	if len(openaiResp.Choices) > 0 {
		choice := openaiResp.Choices[0]

		if choice.Message != nil {
			anthropicResp.Content = convertOpenAIMessageToBlocks(choice.Message)
			anthropicResp.StopReason = convertFinishReason(choice.FinishReason)
		}
	}

	if openaiResp.Usage != nil {
		anthropicResp.Usage = AnthropicUsage{
			InputTokens:  openaiResp.Usage.PromptTokens,
			OutputTokens: openaiResp.Usage.CompletionTokens,
		}
	}

	return anthropicResp
}

func convertOpenAIMessageToBlocks(msg *OpenAIMessageContent) []AnthropicContentBlock {
	var blocks []AnthropicContentBlock

	reasoning := extractReasoning(msg)
	if reasoning != "" {
		blocks = append(blocks, AnthropicContentBlock{
			Type: "thinking",
			Text: reasoning,
		})
	}

	if msg.Content != "" {
		blocks = append(blocks, AnthropicContentBlock{
			Type: "text",
			Text: msg.Content,
		})
	}

	for _, tc := range msg.ToolCalls {
		var input any
		json.Unmarshal([]byte(tc.Function.Arguments), &input)

		blocks = append(blocks, AnthropicContentBlock{
			Type:  "tool_use",
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: input,
		})
	}

	return blocks
}

func extractReasoning(msg *OpenAIMessageContent) string {
	var result string
	if msg.Reasoning != "" {
		result = msg.Reasoning
	} else if msg.ReasoningContent != "" {
		result = msg.ReasoningContent
	} else if len(msg.ReasoningDetails) > 0 {
		var parts []string
		for _, detail := range msg.ReasoningDetails {
			if detail.Content != "" {
				parts = append(parts, detail.Content)
			} else if detail.Summary != "" {
				parts = append(parts, detail.Summary)
			}
		}
		result = strings.Join(parts, "\n")
	}
	if strings.TrimSpace(result) == "" {
		return ""
	}
	return result
}

func convertFinishReason(reason string) string {
	switch reason {
	case "stop":
		return "end_turn"
	case "length":
		return "max_tokens"
	case "tool_calls":
		return "tool_use"
	case "content_filter":
		return "end_turn"
	default:
		return "end_turn"
	}
}
