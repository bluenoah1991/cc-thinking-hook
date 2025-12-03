package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

//go:embed index.html
var indexHTML []byte

func rootHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.Write(indexHTML)
}

func proxyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, err)
		return
	}

	var anthropicReq AnthropicRequest
	if err := json.Unmarshal(body, &anthropicReq); err != nil {
		writeError(w, err)
		return
	}

	originalModel := anthropicReq.Model

	result, err := convertRequest(&anthropicReq)
	if err != nil {
		writeError(w, err)
		return
	}

	saveDiagnosticRequest(body, result)

	if result.IsAnthropic {
		handleAnthropicRequest(w, result.AnthropicRequest)
		return
	}

	openaiBody, err := json.Marshal(result.OpenAIRequest)
	if err != nil {
		writeError(w, err)
		return
	}

	targetURL := backendURL
	if result.UseMultimodal {
		targetURL = multimodalURL
	}

	req, err := http.NewRequest(http.MethodPost, targetURL+"/chat/completions", bytes.NewReader(openaiBody))
	if err != nil {
		writeError(w, err)
		return
	}

	req.Header.Set("Content-Type", "application/json")

	apiKey := resolveAPIKey(r, result.UseMultimodal)
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		writeError(w, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		w.Write(body)
		return
	}

	handleStreamingResponse(w, resp, originalModel)
}

func handleAnthropicRequest(w http.ResponseWriter, anthropicReq *AnthropicRequest) {
	reqBody, err := json.Marshal(anthropicReq)
	if err != nil {
		writeError(w, err)
		return
	}

	req, err := http.NewRequest(http.MethodPost, multimodalURL+"/v1/messages", bytes.NewReader(reqBody))
	if err != nil {
		writeError(w, err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", multimodalAPIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		writeError(w, err)
		return
	}
	defer resp.Body.Close()

	for h, v := range resp.Header {
		lower := strings.ToLower(h)
		if lower != "connection" && lower != "transfer-encoding" {
			w.Header()[h] = v
		}
	}

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func countTokensHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, err)
		return
	}

	if anthropicURL != "" {
		proxyCountTokens(w, body)
		return
	}

	textLength := len(body)
	estimatedTokens := (textLength + 3) / 4

	response := CountTokensResponse{
		InputTokens: estimatedTokens,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	logsMu.Lock()
	logsCopy := make([]string, len(logs))
	copy(logsCopy, logs)
	logsMu.Unlock()

	tokenCount := "estimate"
	if anthropicURL != "" {
		tokenCount = "proxy"
	}

	data := map[string]any{
		"local":      fmt.Sprintf("http://localhost:%d", serverPort),
		"backend":    backendURL,
		"diagnostic": diagnosticMode,
		"ultrathink": ultrathinkPrompt != "",
		"tokencount": tokenCount,
		"multimodal": multimodalURL != "",
		"keeprounds": keepRounds,
		"logs":       logsCopy,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func shutdownHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.WriteHeader(http.StatusOK)
	go func() {
		time.Sleep(100 * time.Millisecond)
		os.Exit(0)
	}()
}

func proxyCountTokens(w http.ResponseWriter, body []byte) {
	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, err)
		return
	}

	req["model"] = anthropicModel

	newBody, err := json.Marshal(req)
	if err != nil {
		writeError(w, err)
		return
	}

	proxyReq, err := http.NewRequest(http.MethodPost, anthropicURL+"/v1/messages/count_tokens", bytes.NewReader(newBody))
	if err != nil {
		writeError(w, err)
		return
	}

	proxyReq.Header.Set("Content-Type", "application/json")
	proxyReq.Header.Set("x-api-key", anthropicAPIKey)
	proxyReq.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{}
	resp, err := client.Do(proxyReq)
	if err != nil {
		writeError(w, err)
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	w.Write(respBody)
}

func resolveAPIKey(r *http.Request, useMultimodal bool) string {
	if useMultimodal {
		return multimodalAPIKey
	}
	if backendAPIKey != "" {
		return backendAPIKey
	}
	if key := r.Header.Get("x-api-key"); key != "" {
		return key
	}
	if auth := r.Header.Get("Authorization"); auth != "" {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}

func writeError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{
			"type":    "api_error",
			"message": err.Error(),
		},
	})
}
