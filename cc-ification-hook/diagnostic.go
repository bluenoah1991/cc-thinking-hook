package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

func saveDiagnosticRequest(anthropicBody []byte, openaiReq *OpenAIRequest) {
	if !diagnosticMode {
		return
	}

	if err := os.MkdirAll("diagnostic", 0755); err != nil {
		addLog(fmt.Sprintf("[✗] Failed to create diagnostic directory: %v", err))
		return
	}

	timestamp := time.Now().Format("20060102_150405_000")

	var anthropicReq interface{}
	if err := json.Unmarshal(anthropicBody, &anthropicReq); err != nil {
		addLog(fmt.Sprintf("[✗] Failed to unmarshal anthropic request: %v", err))
		return
	}

	anthropicReqBody, err := json.MarshalIndent(anthropicReq, "", "  ")
	if err != nil {
		addLog(fmt.Sprintf("[✗] Failed to marshal anthropic request: %v", err))
		return
	}

	anthropicFile := filepath.Join("diagnostic", fmt.Sprintf("anthropic_%s.json", timestamp))
	if err := os.WriteFile(anthropicFile, anthropicReqBody, 0644); err != nil {
		addLog(fmt.Sprintf("[✗] Failed to save anthropic request: %v", err))
	} else {
		addLog(fmt.Sprintf("[📋] Saved anthropic request: %s", anthropicFile))
	}

	openaiBody, err := json.MarshalIndent(openaiReq, "", "  ")
	if err != nil {
		addLog(fmt.Sprintf("[✗] Failed to marshal openai request: %v", err))
		return
	}

	openaiFile := filepath.Join("diagnostic", fmt.Sprintf("openai_%s.json", timestamp))
	if err := os.WriteFile(openaiFile, openaiBody, 0644); err != nil {
		addLog(fmt.Sprintf("[✗] Failed to save openai request: %v", err))
	} else {
		addLog(fmt.Sprintf("[📋] Saved openai request: %s", openaiFile))
	}
}

type StreamRecorder struct {
	file      *os.File
	mu        sync.Mutex
	timestamp string
}

func newStreamRecorder() *StreamRecorder {
	if !diagnosticMode {
		return nil
	}

	if err := os.MkdirAll("diagnostic", 0755); err != nil {
		addLog(fmt.Sprintf("[✗] Failed to create diagnostic directory: %v", err))
		return nil
	}

	timestamp := time.Now().Format("20060102_150405_000")
	filename := filepath.Join("diagnostic", fmt.Sprintf("stream_%s.txt", timestamp))

	file, err := os.Create(filename)
	if err != nil {
		addLog(fmt.Sprintf("[✗] Failed to create stream file: %v", err))
		return nil
	}

	addLog(fmt.Sprintf("[📋] Recording stream to: %s", filename))
	return &StreamRecorder{
		file:      file,
		timestamp: timestamp,
	}
}

func (r *StreamRecorder) RecordChunk(line string) {
	if r == nil || r.file == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.file.WriteString(line + "\n")
}

func (r *StreamRecorder) Close() {
	if r == nil || r.file == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.file.Close()
	addLog(fmt.Sprintf("[📋] Stream recording completed: %s", r.timestamp))
}
