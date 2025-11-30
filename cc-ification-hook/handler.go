package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

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

	openaiReq, err := convertAnthropicToOpenAI(&anthropicReq)
	if err != nil {
		writeError(w, err)
		return
	}

	saveDiagnosticRequest(body, openaiReq)

	openaiBody, err := json.Marshal(openaiReq)
	if err != nil {
		writeError(w, err)
		return
	}

	req, err := http.NewRequest(http.MethodPost, backendURL+"/chat/completions", bytes.NewReader(openaiBody))
	if err != nil {
		writeError(w, err)
		return
	}

	req.Header.Set("Content-Type", "application/json")

	apiKey := resolveAPIKey(r)
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

func resolveAPIKey(r *http.Request) string {
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
