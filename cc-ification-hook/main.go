package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
)

var (
	backendURL       string
	backendAPIKey    string
	backendModel     string
	diagnosticMode   bool
	ultrathinkPrompt string
	anthropicURL     string
	anthropicAPIKey  string
	anthropicModel   string
)

func main() {
	diagnostic := flag.Bool("diagnostic", false, "Enable diagnostic mode")
	flag.BoolVar(diagnostic, "d", false, "Enable diagnostic mode")
	port := flag.Int("port", 5281, "Port to run the proxy on")
	flag.IntVar(port, "p", 5281, "Port to run the proxy on")
	flag.Parse()

	diagnosticMode = *diagnostic

	loadUltrathinkPrompt()
	loadAnthropicConfig()

	backendURL = getInput("Backend OpenAI API URL: ", true)
	backendURL = strings.TrimRight(backendURL, "/")
	backendAPIKey = getInput("Backend API Key (optional, uses original if empty): ", false)
	backendModel = getInput("Backend Model (optional, uses original if empty): ", false)

	fmt.Println()
	fmt.Println("🚀 CC-ification Hook")
	fmt.Printf("   Local:   http://localhost:%d\n", *port)
	fmt.Printf("   Backend: %s\n", backendURL)
	if diagnosticMode {
		fmt.Println("   📋 Diagnostic: enabled")
	}
	if ultrathinkPrompt != "" {
		fmt.Println("   🧠 UltraThink: enabled")
	}
	if anthropicURL != "" {
		fmt.Println("   📊 TokenCount: proxy")
	} else {
		fmt.Println("   📊 TokenCount: estimate")
	}
	fmt.Printf("\n   export ANTHROPIC_BASE_URL=http://localhost:%d\n", *port)
	fmt.Println("\n   Press Ctrl+C to stop")
	fmt.Println()

	http.HandleFunc("/", handleRoot)
	http.HandleFunc("/v1/messages", proxyHandler)
	http.HandleFunc("/v1/messages/count_tokens", countTokensHandler)

	if err := http.ListenAndServe(fmt.Sprintf(":%d", *port), nil); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}

func loadUltrathinkPrompt() {
	data, err := os.ReadFile("ultrathink.txt")
	if err != nil {
		return
	}
	prompt := strings.TrimSpace(string(data))
	if prompt != "" {
		ultrathinkPrompt = prompt
		fmt.Println("[✓] Loaded ultrathink.txt")
	}
}

func loadAnthropicConfig() {
	data, err := os.ReadFile("anthropic.json")
	if err != nil {
		return
	}
	var config struct {
		URL    string `json:"url"`
		APIKey string `json:"api_key"`
		Model  string `json:"model"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return
	}
	if config.URL != "" && config.APIKey != "" && config.Model != "" {
		anthropicURL = strings.TrimRight(config.URL, "/")
		anthropicAPIKey = config.APIKey
		anthropicModel = config.Model
		fmt.Println("[✓] Loaded anthropic.json")
	}
}

func getInput(prompt string, required bool) string {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print(prompt)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input != "" || !required {
			return input
		}
		fmt.Println("[✗] This field is required.")
	}
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok","service":"cc-ification-hook"}`))
}
