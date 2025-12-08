package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func handleStreamingResponse(w http.ResponseWriter, resp *http.Response, originalModel string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	state := &StreamState{
		MessageID:       fmt.Sprintf("msg_%d", time.Now().UnixNano()),
		Model:           originalModel,
		CurrentIndex:    0,
		TextStarted:     false,
		TextIndex:       -1,
		ThinkingStarted: false,
		ThinkingIndex:   -1,
		ToolCalls:       make(map[int]*ToolCallState),
		Interceptor:     CreateStreamInterceptor(),
	}

	recorder := newStreamRecorder()
	defer func() {
		if recorder != nil {
			recorder.Close()
		}
	}()

	sendEvent(w, "message_start", map[string]any{
		"type": "message_start",
		"message": map[string]any{
			"id":            state.MessageID,
			"type":          "message",
			"role":          "assistant",
			"content":       []any{},
			"model":         state.Model,
			"stop_reason":   nil,
			"stop_sequence": nil,
			"usage": map[string]any{
				"input_tokens":  100,
				"output_tokens": 1,
			},
		},
	})
	flusher.Flush()

	reader := bufio.NewReader(resp.Body)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				addLog(fmt.Sprintf("[✗] Stream read error: %v", err))
			}
			break
		}

		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "data: ") {
			continue
		}

		recorder.RecordChunk(line)

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			recorder.RecordChunk("[DONE]")
			break
		}

		var chunk OpenAIResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if chunk.Usage != nil {
			state.AccumulatedUsage = chunk.Usage
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		choice := chunk.Choices[0]

		if choice.Delta != nil {
			processStreamDelta(w, flusher, state, choice.Delta)
		}

		if choice.FinishReason == "tool_calls" {
			for _, tc := range state.ToolCalls {
				if tc.Started && !tc.Closed {
					sendEvent(w, "content_block_stop", map[string]any{
						"type":  "content_block_stop",
						"index": tc.BlockIndex,
					})
					tc.Closed = true
				}
			}
			flusher.Flush()
		}

		if choice.FinishReason != "" {
			finalizeStream(w, flusher, state, choice.FinishReason)
			state.Finalized = true
			break
		}
	}

	if !state.Finalized {
		finalizeStream(w, flusher, state, "end_turn")
	}
}

func processStreamDelta(w http.ResponseWriter, flusher http.Flusher, state *StreamState, delta *OpenAIDelta) {
	if state.Interceptor != nil {
		state.Interceptor.OnDeltaStart(delta)
	}

	reasoning := extractStreamReasoning(delta)
	if reasoning != "" {
		handleThinkingDelta(w, flusher, state, reasoning)
	}

	if delta.Content != "" {
		handleTextDelta(w, flusher, state, delta.Content)
	}

	if len(delta.ToolCalls) > 0 {
		for _, tc := range delta.ToolCalls {
			idx := tc.Index
			toolState := state.ToolCalls[idx]

			if tc.Function.Name != "" {
				if toolState == nil {
					if state.ThinkingStarted {
						sendEvent(w, "content_block_stop", map[string]any{
							"type":  "content_block_stop",
							"index": state.ThinkingIndex,
						})
						state.ThinkingStarted = false
					}

					if state.TextStarted {
						sendEvent(w, "content_block_stop", map[string]any{
							"type":  "content_block_stop",
							"index": state.TextIndex,
						})
						state.TextStarted = false
					}

					toolState = &ToolCallState{
						ID:         tc.ID,
						Name:       tc.Function.Name,
						BlockIndex: state.CurrentIndex,
						Started:    false,
						Closed:     false,
					}
					state.ToolCalls[idx] = toolState
					state.CurrentIndex++
				}

				if !toolState.Started {
					sendEvent(w, "content_block_start", map[string]any{
						"type":  "content_block_start",
						"index": toolState.BlockIndex,
						"content_block": map[string]any{
							"type": "tool_use",
							"id":   toolState.ID,
							"name": toolState.Name,
						},
					})
					toolState.Started = true
				}
			}

			if tc.Function.Arguments != "" && toolState != nil {
				sendEvent(w, "content_block_delta", map[string]any{
					"type":  "content_block_delta",
					"index": toolState.BlockIndex,
					"delta": map[string]any{
						"type":         "input_json_delta",
						"partial_json": tc.Function.Arguments,
					},
				})
			}
		}
		flusher.Flush()
	}
}

func handleThinkingDelta(w http.ResponseWriter, flusher http.Flusher, state *StreamState, reasoning string) {
	if !state.ThinkingStarted {
		if reasoning == "\n" {
			return
		}
		state.ThinkingIndex = state.CurrentIndex
		state.CurrentIndex++
		sendEvent(w, "content_block_start", map[string]any{
			"type":  "content_block_start",
			"index": state.ThinkingIndex,
			"content_block": map[string]any{
				"type":     "thinking",
				"thinking": "",
			},
		})
		state.ThinkingStarted = true
	}
	sendEvent(w, "content_block_delta", map[string]any{
		"type":  "content_block_delta",
		"index": state.ThinkingIndex,
		"delta": map[string]any{
			"type":     "thinking_delta",
			"thinking": reasoning,
		},
	})
	flusher.Flush()
}

func handleTextDelta(w http.ResponseWriter, flusher http.Flusher, state *StreamState, content string) {
	if state.ThinkingStarted && state.ThinkingIndex >= 0 {
		sendEvent(w, "content_block_stop", map[string]any{
			"type":  "content_block_stop",
			"index": state.ThinkingIndex,
		})
		state.ThinkingStarted = false
	}

	if !state.TextStarted {
		if content == "\n" {
			return
		}
		state.TextIndex = state.CurrentIndex
		state.CurrentIndex++
		sendEvent(w, "content_block_start", map[string]any{
			"type":  "content_block_start",
			"index": state.TextIndex,
			"content_block": map[string]any{
				"type": "text",
				"text": "",
			},
		})
		state.TextStarted = true
	}
	sendEvent(w, "content_block_delta", map[string]any{
		"type":  "content_block_delta",
		"index": state.TextIndex,
		"delta": map[string]any{
			"type": "text_delta",
			"text": content,
		},
	})
	flusher.Flush()
}

func finalizeStream(w http.ResponseWriter, flusher http.Flusher, state *StreamState, reason string) {
	if state.ThinkingStarted {
		sendEvent(w, "content_block_stop", map[string]any{
			"type":  "content_block_stop",
			"index": state.ThinkingIndex,
		})
	}

	if state.TextStarted {
		sendEvent(w, "content_block_stop", map[string]any{
			"type":  "content_block_stop",
			"index": state.TextIndex,
		})
	}

	for _, tc := range state.ToolCalls {
		if tc.Started && !tc.Closed {
			sendEvent(w, "content_block_stop", map[string]any{
				"type":  "content_block_stop",
				"index": tc.BlockIndex,
			})
			tc.Closed = true
		}
	}

	outputTokens := 0
	promptTokens := 0
	cachedTokens := 0
	responseTotalTokens := 0
	if state.AccumulatedUsage != nil {
		promptTokens = int(float64(state.AccumulatedUsage.PromptTokens) * tokenScaleFactor)
		outputTokens = int(float64(state.AccumulatedUsage.CompletionTokens) * tokenScaleFactor)
		responseTotalTokens = int(float64(state.AccumulatedUsage.TotalTokens) * tokenScaleFactor)
		if state.AccumulatedUsage.PromptTokensDetails != nil {
			cachedTokens = int(float64(state.AccumulatedUsage.PromptTokensDetails.CachedTokens) * tokenScaleFactor)
		}
	}

	statsMu.Lock()
	totalPromptTokens += int64(promptTokens)
	totalCompletionTokens += int64(outputTokens)
	totalCachedTokens += int64(cachedTokens)
	totalTokens += int64(responseTotalTokens)
	statsMu.Unlock()

	if diagnosticMode {
		addLog(fmt.Sprintf("[✓] Output tokens: %d (scale: %.2f)", outputTokens, tokenScaleFactor))
	}

	sendEvent(w, "message_delta", map[string]any{
		"type": "message_delta",
		"delta": map[string]any{
			"stop_reason":   convertFinishReason(reason),
			"stop_sequence": nil,
		},
		"usage": map[string]any{
			"output_tokens": outputTokens,
		},
	})

	sendEvent(w, "message_stop", map[string]any{
		"type": "message_stop",
	})

	flusher.Flush()
}

func extractStreamReasoning(delta *OpenAIDelta) string {
	var result string
	if delta.Reasoning != "" {
		result = delta.Reasoning
	} else if delta.ReasoningContent != "" {
		result = delta.ReasoningContent
	} else if len(delta.ReasoningDetails) > 0 {
		var parts []string
		for _, detail := range delta.ReasoningDetails {
			if detail.Content != "" {
				parts = append(parts, detail.Content)
			}
		}
		result = strings.Join(parts, "")
	}
	return result
}

func sendEvent(w http.ResponseWriter, event string, data any) {
	jsonData, _ := json.Marshal(data)
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, jsonData)
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
