package main

type AnthropicMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type AnthropicThinking struct {
	Type         string `json:"type"`
	BudgetTokens int    `json:"budget_tokens"`
}

type AnthropicTool struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	InputSchema any    `json:"input_schema"`
}

type AnthropicToolChoice struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}

type AnthropicRequest struct {
	Model         string               `json:"model"`
	Messages      []AnthropicMessage   `json:"messages"`
	System        any                  `json:"system,omitempty"`
	MaxTokens     int                  `json:"max_tokens,omitempty"`
	Temperature   *float64             `json:"temperature,omitempty"`
	TopP          *float64             `json:"top_p,omitempty"`
	TopK          *int                 `json:"top_k,omitempty"`
	StopSequences []string             `json:"stop_sequences,omitempty"`
	Stream        bool                 `json:"stream,omitempty"`
	Tools         []AnthropicTool      `json:"tools,omitempty"`
	ToolChoice    *AnthropicToolChoice `json:"tool_choice,omitempty"`
	Thinking      *AnthropicThinking   `json:"thinking,omitempty"`
}

type ImageURL struct {
	URL string `json:"url"`
}

type OpenAIContentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

type ToolCallFunction struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type OpenAIToolCall struct {
	Index    int              `json:"index,omitempty"`
	ID       string           `json:"id,omitempty"`
	Type     string           `json:"type,omitempty"`
	Function ToolCallFunction `json:"function"`
}

type ToolFunction struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters"`
}

type OpenAITool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type OpenAIMessage struct {
	Role             string           `json:"role"`
	Content          any              `json:"content"`
	ToolCalls        []OpenAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string           `json:"tool_call_id,omitempty"`
	ReasoningContent string           `json:"reasoning_content,omitempty"`
}

type StreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type OpenAIRequest struct {
	Model           string          `json:"model"`
	Messages        []OpenAIMessage `json:"messages"`
	MaxTokens       int             `json:"max_tokens,omitempty"`
	Temperature     *float64        `json:"temperature,omitempty"`
	TopP            *float64        `json:"top_p,omitempty"`
	Stop            []string        `json:"stop,omitempty"`
	Stream          bool            `json:"stream,omitempty"`
	Tools           []OpenAITool    `json:"tools,omitempty"`
	ToolChoice      any             `json:"tool_choice,omitempty"`
	StreamOptions   *StreamOptions  `json:"stream_options,omitempty"`
	ReasoningEffort string          `json:"reasoning_effort,omitempty"`
}

type ReasoningDetail struct {
	Type    string `json:"type"`
	Content string `json:"content,omitempty"`
	Summary string `json:"summary,omitempty"`
}

type OpenAIDelta struct {
	Role             string            `json:"role,omitempty"`
	Content          string            `json:"content,omitempty"`
	ToolCalls        []OpenAIToolCall  `json:"tool_calls,omitempty"`
	Reasoning        string            `json:"reasoning,omitempty"`
	ReasoningContent string            `json:"reasoning_content,omitempty"`
	ReasoningDetails []ReasoningDetail `json:"reasoning_details,omitempty"`
}

type OpenAIChoice struct {
	Index        int          `json:"index"`
	Delta        *OpenAIDelta `json:"delta,omitempty"`
	FinishReason string       `json:"finish_reason,omitempty"`
}

type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type OpenAIResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []OpenAIChoice `json:"choices"`
	Usage   *OpenAIUsage   `json:"usage,omitempty"`
}

type OpenAINonStreamMessage struct {
	Role             string            `json:"role"`
	Content          string            `json:"content"`
	ToolCalls        []OpenAIToolCall  `json:"tool_calls,omitempty"`
	Reasoning        string            `json:"reasoning,omitempty"`
	ReasoningContent string            `json:"reasoning_content,omitempty"`
	ReasoningDetails []ReasoningDetail `json:"reasoning_details,omitempty"`
}

type OpenAINonStreamChoice struct {
	Index        int                    `json:"index"`
	Message      OpenAINonStreamMessage `json:"message"`
	FinishReason string                 `json:"finish_reason"`
}

type OpenAINonStreamResponse struct {
	ID      string                  `json:"id"`
	Object  string                  `json:"object"`
	Created int64                   `json:"created"`
	Model   string                  `json:"model"`
	Choices []OpenAINonStreamChoice `json:"choices"`
	Usage   *OpenAIUsage            `json:"usage,omitempty"`
}

type AnthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type AnthropicResponse struct {
	ID           string          `json:"id"`
	Type         string          `json:"type"`
	Role         string          `json:"role"`
	Content      []any           `json:"content"`
	Model        string          `json:"model"`
	StopReason   string          `json:"stop_reason"`
	StopSequence *string         `json:"stop_sequence"`
	Usage        *AnthropicUsage `json:"usage"`
}

type StreamState struct {
	MessageID        string
	Model            string
	CurrentIndex     int
	TextStarted      bool
	TextIndex        int
	ThinkingStarted  bool
	ThinkingIndex    int
	ToolCalls        map[int]*ToolCallState
	AccumulatedUsage *OpenAIUsage
	Finalized        bool
	Interceptor      Interceptor
}

type ToolCallState struct {
	ID         string
	Name       string
	BlockIndex int
	Started    bool
	Closed     bool
}

type CountTokensResponse struct {
	InputTokens int `json:"input_tokens"`
}

type CompressionStats struct {
	ThinkingBlocks int
	ToolCalls      int
	ToolResults    int
}

type ConvertResult struct {
	AnthropicRequest *AnthropicRequest
	OpenAIRequest    *OpenAIRequest
	UseMultimodal    bool
	IsAnthropic      bool
}
