package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

var (
	backendURL       string
	ultrathinkPrompt string
	diagnosticMode   bool
)

func logMessage(status int) {
	if status >= 400 {
		fmt.Printf("[✗] HTTP %d\n", status)
	}
}

func saveDiagnosticRequest(data []byte) {
	if !diagnosticMode {
		return
	}

	if err := os.MkdirAll("diagnostic", 0755); err != nil {
		fmt.Printf("[✗] Failed to save diagnostic: %v\n", err)
		return
	}

	filename := fmt.Sprintf("request_%s.json", time.Now().Format("20060102_150405_000"))
	filepath := filepath.Join("diagnostic", filename)

	if err := os.WriteFile(filepath, data, 0644); err != nil {
		fmt.Printf("[✗] Failed to save diagnostic: %v\n", err)
		return
	}

	fmt.Printf("[📋] Diagnostic saved: %s\n", filename)
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
		if len(text) > 20 {
			preview = text[:20] + "..."
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
			if len(text) > 20 {
				preview = text[:20] + "..."
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

	fmt.Printf("[✓] Injected prompt: %s\n", preview)
	return modified, true
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

func writeError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	fmt.Fprintf(w, `{"error":"%s"}`, err.Error())
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

func main() {
	diagnostic := flag.Bool("diagnostic", false, "Enable diagnostic mode")
	flag.BoolVar(diagnostic, "d", false, "Enable diagnostic mode")
	port := flag.Int("port", 5280, "Port to run the proxy on")
	flag.IntVar(port, "p", 5280, "Port to run the proxy on")
	urlFlag := flag.String("url", "", "Backend API URL")
	flag.StringVar(urlFlag, "u", "", "Backend API URL")
	flag.Parse()

	diagnosticMode = *diagnostic
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
	fmt.Printf("   Local:   http://localhost:%d\n", *port)
	fmt.Printf("   Backend: %s\n", backendURL)
	if diagnosticMode {
		fmt.Println("   📋 Diagnostic: enabled (saving to 'diagnostic/' directory)")
	}
	fmt.Printf("\n   export ANTHROPIC_BASE_URL=http://localhost:%d\n", *port)
	fmt.Println("\n   Press Ctrl+C to stop")
	fmt.Println()

	http.HandleFunc("/", proxyHandler)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", *port), nil); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}
