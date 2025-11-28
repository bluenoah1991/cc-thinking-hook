package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func saveDiagnosticRequest(anthropicBody []byte, openaiReq *OpenAIRequest) {
	if !diagnosticMode {
		return
	}

	if err := os.MkdirAll("diagnostic", 0755); err != nil {
		fmt.Printf("[✗] Failed to create diagnostic directory: %v\n", err)
		return
	}

	timestamp := time.Now().Format("20060102_150405_000")

	var anthropicReq interface{}
	if err := json.Unmarshal(anthropicBody, &anthropicReq); err != nil {
		fmt.Printf("[✗] Failed to unmarshal anthropic request: %v\n", err)
		return
	}

	anthropicReqBody, err := json.MarshalIndent(anthropicReq, "", "  ")
	if err != nil {
		fmt.Printf("[✗] Failed to marshal anthropic request: %v\n", err)
		return
	}

	anthropicFile := filepath.Join("diagnostic", fmt.Sprintf("anthropic_%s.json", timestamp))
	if err := os.WriteFile(anthropicFile, anthropicReqBody, 0644); err != nil {
		fmt.Printf("[✗] Failed to save anthropic request: %v\n", err)
	} else {
		fmt.Printf("[📋] Saved anthropic request: %s\n", anthropicFile)
	}

	openaiBody, err := json.MarshalIndent(openaiReq, "", "  ")
	if err != nil {
		fmt.Printf("[✗] Failed to marshal openai request: %v\n", err)
		return
	}

	openaiFile := filepath.Join("diagnostic", fmt.Sprintf("openai_%s.json", timestamp))
	if err := os.WriteFile(openaiFile, openaiBody, 0644); err != nil {
		fmt.Printf("[✗] Failed to save openai request: %v\n", err)
	} else {
		fmt.Printf("[📋] Saved openai request: %s\n", openaiFile)
	}
}
