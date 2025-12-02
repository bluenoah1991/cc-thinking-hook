package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
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
	tokenScaleFactor float64
	serverPort       int
	keepRounds       int
	logs             []string
	logsMu           sync.Mutex
)

func main() {
	diagnostic := flag.Bool("diagnostic", false, "Enable diagnostic mode")
	flag.BoolVar(diagnostic, "d", false, "Enable diagnostic mode")
	port := flag.Int("port", 5281, "Port to run the proxy on")
	flag.IntVar(port, "p", 5281, "Port to run the proxy on")
	urlFlag := flag.String("url", "", "Backend OpenAI API URL")
	flag.StringVar(urlFlag, "u", "", "Backend OpenAI API URL")
	keyFlag := flag.String("key", "", "Backend API Key (optional)")
	flag.StringVar(keyFlag, "k", "", "Backend API Key (optional)")
	modelFlag := flag.String("model", "", "Backend Model (optional)")
	flag.StringVar(modelFlag, "m", "", "Backend Model (optional)")
	scaleFlag := flag.Float64("scale", 1.0, "Token scale factor")
	flag.Float64Var(scaleFlag, "s", 1.0, "Token scale factor")
	roundFlag := flag.Int("round", 0, "Keep recent N rounds uncompressed")
	flag.IntVar(roundFlag, "r", 0, "Keep recent N rounds uncompressed")
	flag.Parse()

	diagnosticMode = *diagnostic
	tokenScaleFactor = *scaleFlag
	serverPort = *port
	keepRounds = *roundFlag

	loadUltrathinkPrompt()
	loadAnthropicConfig()

	if *urlFlag != "" {
		backendURL = strings.TrimRight(*urlFlag, "/")
	} else {
		backendURL = getInput("Backend OpenAI API URL: ", true)
		backendURL = strings.TrimRight(backendURL, "/")
	}
	if *keyFlag != "" {
		backendAPIKey = *keyFlag
	} else if *urlFlag == "" {
		backendAPIKey = getInput("Backend API Key (optional, uses original if empty): ", false)
	}
	if *modelFlag != "" {
		backendModel = *modelFlag
	} else if *urlFlag == "" {
		backendModel = getInput("Backend Model (optional, uses original if empty): ", false)
	}

	InitInterceptor()

	fmt.Println()
	fmt.Println("🚀 CC-ification Hook")
	fmt.Printf("   Local:   http://localhost:%d\n", serverPort)
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
	if keepRounds > 0 {
		fmt.Printf("   📦 Compress: keep %d rounds\n", keepRounds)
	}
	fmt.Printf("\n   export ANTHROPIC_BASE_URL=http://localhost:%d\n", serverPort)
	fmt.Println("\n   Press Ctrl+C to stop")
	fmt.Println()

	http.HandleFunc("/", rootHandler)
	http.HandleFunc("/v1/messages", proxyHandler)
	http.HandleFunc("/v1/messages/count_tokens", countTokensHandler)
	http.HandleFunc("/status", statusHandler)
	http.HandleFunc("/shutdown", shutdownHandler)

	if err := http.ListenAndServe(fmt.Sprintf(":%d", serverPort), nil); err != nil {
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

func addLog(msg string) {
	logsMu.Lock()
	defer logsMu.Unlock()
	logs = append(logs, time.Now().Format("15:04:05")+" "+msg)
	if len(logs) > 100 {
		logs = logs[1:]
	}
	fmt.Println(msg)
}
