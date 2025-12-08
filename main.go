package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

//go:embed index.html
var indexHTML []byte

var (
	backendURL       string
	ultrathinkPrompt string
	diagnosticMode   bool
	serverPort       int
	logs             []string
	logsMu           sync.Mutex
)

func main() {
	diagnostic := flag.Bool("diagnostic", false, "Enable diagnostic mode")
	flag.BoolVar(diagnostic, "d", false, "Enable diagnostic mode")
	port := flag.Int("port", 5280, "Port to run the proxy on")
	flag.IntVar(port, "p", 5280, "Port to run the proxy on")
	urlFlag := flag.String("url", "", "Backend API URL")
	flag.StringVar(urlFlag, "u", "", "Backend API URL")
	flag.Parse()

	diagnosticMode = *diagnostic
	serverPort = *port
	if *urlFlag != "" {
		backendURL = strings.TrimRight(*urlFlag, "/")
	} else {
		backendURL = getBackendURL()
	}

	data, err := os.ReadFile("ultrathink.txt")
	if err != nil {
		fmt.Println("[✗] Could not load ultrathink.txt")
	} else {
		ultrathinkPrompt = string(data)
	}

	fmt.Println()
	fmt.Println("🚀 Claude UltraThink Proxy")
	fmt.Printf("   Local:   http://localhost:%d\n", serverPort)
	fmt.Printf("   Backend: %s\n", backendURL)
	if diagnosticMode {
		fmt.Println("   📋 Diagnostic: enabled (saving to 'diagnostic/' directory)")
	}
	fmt.Printf("\n   export ANTHROPIC_BASE_URL=http://localhost:%d\n", serverPort)
	fmt.Println("\n   Press Ctrl+C to stop")
	fmt.Println()

	http.HandleFunc("/", proxyHandler)
	http.HandleFunc("/status", statusHandler)
	http.HandleFunc("/shutdown", shutdownHandler)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", serverPort), nil); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}

func getBackendURL() string {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Backend API URL: ")
		url, _ := reader.ReadString('\n')
		url = strings.TrimSpace(url)
		if url != "" {
			return strings.TrimRight(url, "/")
		}
		fmt.Println("[✗] Backend URL cannot be empty.")
	}
}

func proxyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		w.Header().Set("Content-Type", "text/html")
		w.Write(indexHTML)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, err)
		return
	}

	modified, injected := injectUltrathink(body)
	if injected {
		saveDiagnosticRequest(modified)
	}

	req, err := http.NewRequest(http.MethodPost, backendURL, bytes.NewReader(modified))
	if err != nil {
		writeError(w, err)
		return
	}

	for h, v := range r.Header {
		lower := strings.ToLower(h)
		if lower != "host" && lower != "content-length" {
			req.Header[h] = v
		}
	}
	req.Header.Set("Content-Length", fmt.Sprintf("%d", len(modified)))

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{},
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		writeError(w, err)
		return
	}
	defer resp.Body.Close()

	logMessage(resp.StatusCode)

	for h, v := range resp.Header {
		lower := strings.ToLower(h)
		if lower != "connection" && lower != "transfer-encoding" {
			w.Header()[h] = v
		}
	}

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	logsMu.Lock()
	logsCopy := make([]string, len(logs))
	copy(logsCopy, logs)
	logsMu.Unlock()

	data := map[string]any{
		"local":      fmt.Sprintf("http://localhost:%d", serverPort),
		"backend":    backendURL,
		"diagnostic": diagnosticMode,
		"ultrathink": ultrathinkPrompt != "",
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

func injectUltrathink(body []byte) ([]byte, bool) {
	if ultrathinkPrompt == "" {
		return body, false
	}

	if !gjson.GetBytes(body, "thinking").Exists() {
		return body, false
	}

	messages := gjson.GetBytes(body, "messages")
	if !messages.Exists() || !messages.IsArray() || len(messages.Array()) == 0 {
		return body, false
	}

	lastIdx := len(messages.Array()) - 1
	lastMsg := messages.Array()[lastIdx]

	if lastMsg.Get("role").String() != "user" {
		return body, false
	}

	content := lastMsg.Get("content")
	contentPath := fmt.Sprintf("messages.%d.content", lastIdx)

	ultrathinkBlock := map[string]string{"type": "text", "text": ultrathinkPrompt}

	var modified []byte
	var err error
	var preview string

	if content.Type == gjson.String {
		newContent := []any{
			map[string]string{"type": "text", "text": content.String()},
			ultrathinkBlock,
		}
		modified, err = sjson.SetBytes(body, contentPath, newContent)
		text := content.String()
		if len(text) > 100 {
			preview = text[:100] + "..."
		} else {
			preview = text
		}
	} else if content.IsArray() {
		for _, block := range content.Array() {
			if block.Get("type").String() != "text" {
				return body, false
			}
		}
		modified, err = sjson.SetBytes(body, contentPath+".-1", ultrathinkBlock)
		if len(content.Array()) > 0 {
			text := content.Array()[0].Get("text").String()
			if len(text) > 100 {
				preview = text[:100] + "..."
			} else {
				preview = text
			}
		}
	} else {
		return body, false
	}

	if err != nil {
		return body, false
	}

	addLog(fmt.Sprintf("[✓] Injected prompt: %s", preview))
	return modified, true
}

func saveDiagnosticRequest(data []byte) {
	if !diagnosticMode {
		return
	}

	if err := os.MkdirAll("diagnostic", 0755); err != nil {
		addLog(fmt.Sprintf("[✗] Failed to save diagnostic: %v", err))
		return
	}

	filename := fmt.Sprintf("request_%s.json", time.Now().Format("20060102_150405_000"))
	filepath := filepath.Join("diagnostic", filename)

	if err := os.WriteFile(filepath, data, 0644); err != nil {
		addLog(fmt.Sprintf("[✗] Failed to save diagnostic: %v", err))
		return
	}

	addLog(fmt.Sprintf("[📋] Diagnostic saved: %s", filename))
}

func addLog(msg string) {
	logsMu.Lock()
	defer logsMu.Unlock()
	logs = append(logs, time.Now().Format("15:04:05")+" "+msg)
	if len(logs) > 100 {
		logs = logs[1:]
	}
	fmt.Println(msg)
}

func logMessage(status int) {
	if status >= 400 {
		addLog(fmt.Sprintf("[✗] HTTP %d", status))
	}
}

func writeError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	fmt.Fprintf(w, `{"error":"%s"}`, err.Error())
}
