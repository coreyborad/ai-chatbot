package grok

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

/*
	{
	  "messages": [
	    {
	      "role": "system",
	      "content": "You are a helpful assistant that can answer questions and help with tasks."
	    },
	    {
	      "role": "user",
	      "content": "What is 101*3?"
	    }
	  ],
	  "reasoning_effort": "low",
	  "model": "grok-3-mini-fast-latest"
	}
*/
type GrokCompletionsMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type GrokCompletionsRequest struct {
	Messages []*GrokCompletionsMessage `json:"messages"`
	Model    string                    `json:"model"`
}

type GrokCompletionsResponse struct {
	Choices []struct {
		Index        int    `json:"index"`
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int    `json:"created"`
	Model   string `json:"model"`
	Usage   struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

type ChatBotRequest []*GrokCompletionsMessage

func GrokRoute(w http.ResponseWriter, r *http.Request) {
	// 設定 CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*") // 或指定來源
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// 處理預檢請求（OPTIONS）
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	apiKey := os.Getenv("GROK_API_KEY")
	if apiKey == "" {
		log.Fatal("GROK_API_KEY not set in .env")
	}
	chatbotRequest := &ChatBotRequest{}

	if err := json.NewDecoder(r.Body).Decode(&chatbotRequest); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	grokCompletionsRequest := &GrokCompletionsRequest{
		Messages: *chatbotRequest,
		Model:    "grok-3-beta",
	}

	// Marshal the request to JSON
	jsonData, err := json.Marshal(grokCompletionsRequest)
	if err != nil {
		http.Error(w, "Failed to marshal JSON", http.StatusInternalServerError)
		return
	}

	// Create HTTP request
	url := "https://api.x.ai/v1/chat/completions"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Failed to send request", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Read and parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Failed to read response", http.StatusInternalServerError)
		return
	}

	if resp.StatusCode != http.StatusOK {
		http.Error(w, fmt.Sprintf("Error: %s", body), resp.StatusCode)
		return
	}

	var grokCompletionsResponse GrokCompletionsResponse
	if err := json.Unmarshal(body, &grokCompletionsResponse); err != nil {
		http.Error(w, "Failed to parse response", http.StatusInternalServerError)
		return
	}
	// Send the response back to the client
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(grokCompletionsResponse); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		log.Printf("Failed to encode response: %v", err)
	}
	return
}
