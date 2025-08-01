package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"linebot-grok/utils"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/patrickmn/go-cache"
	"google.golang.org/genai"
)

var imgCache = cache.New(180*time.Minute, 180*time.Minute)

func GenerateImageByGemini(host string, userMsg string) (string, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  os.Getenv("GEMINI_API_KEY"),
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		log.Fatal(err)
	}
	if client.ClientConfig().Backend == genai.BackendVertexAI {
		fmt.Println("Calling VertexAI Backend...")
	} else {
		fmt.Println("Calling GeminiAPI Backend...")
	}
	maxOutputTokens := int32(256)
	config := &genai.GenerateContentConfig{
		HTTPOptions: &genai.HTTPOptions{
			APIVersion: "v1beta",
		},
		MaxOutputTokens:    maxOutputTokens,
		ResponseModalities: []string{"IMAGE", "TEXT"},
	}
	// config.ResponseModalities = []string{"IMAGE", "TEXT"}
	// Call the GenerateContent method.

	promptMsg := userMsg

	result, err := client.Models.GenerateContent(ctx, "gemini-2.0-flash-exp-image-generation", genai.Text(promptMsg), config)
	if err != nil {
		log.Fatal(err)
	}
	// 提取圖片資料
	for _, cand := range result.Candidates {
		if cand.Content != nil {
			for _, part := range cand.Content.Parts {
				if part.InlineData != nil {
					uuid := uuid.New().String()
					imgKey := fmt.Sprintf("%s.png", uuid)
					imgCache.SetDefault(imgKey, part.InlineData.Data)
					return host + "/img/" + imgKey, nil
				}
			}
		}
	}
	jdata, _ := json.MarshalIndent(result, "", "  ")

	fmt.Println("Image data not found in response", string(jdata))
	return "", nil
}

func GenerateByGemini(userMsg string) (string, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  os.Getenv("GEMINI_API_KEY"),
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create Gemini client: %w", err)
	}
	if client.ClientConfig().Backend == genai.BackendVertexAI {
		fmt.Println("Calling VertexAI Backend...")
	} else {
		fmt.Println("Calling GeminiAPI Backend...")
	}
	maxOutputTokens := int32(256)
	config := &genai.GenerateContentConfig{
		HTTPOptions: &genai.HTTPOptions{
			APIVersion: "v1beta",
		},
		MaxOutputTokens:    maxOutputTokens,
		ResponseModalities: []string{"TEXT"},
	}

	promptMsg := userMsg

	result, err := client.Models.GenerateContent(ctx, "gemini-2.0-flash", genai.Text(promptMsg), config)
	if err != nil {
		return "", fmt.Errorf("failed to generate content: %w", err)
	}
	return result.Text(), nil
}

type CompletionsMessage struct {
	Content string `json:"content"`
}

func GeminiRoute(w http.ResponseWriter, r *http.Request) {
	// 設定 CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*") // 或指定來源
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// 處理預檢請求（OPTIONS）
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	chatbotRequest := &CompletionsMessage{}

	if err := json.NewDecoder(r.Body).Decode(&chatbotRequest); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	ip := utils.GetClientIP(r)
	if ip == "" {
		http.Error(w, "Could not determine client IP", http.StatusBadRequest)
		log.Println("Could not determine client IP")
		return
	}
	location := utils.GetLocationByIP(ip)
	fmt.Println(ip, location, "ASDASD")
	resp, err := GenerateByGeminiWithSearch(chatbotRequest.Content, location)
	if err != nil {
		http.Error(w, "Failed to generate response", http.StatusInternalServerError)
		log.Printf("Failed to generate response: %v", err)
		return
	}
	// Send the response back to the client
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"response": resp,
	}); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		log.Printf("Failed to encode response: %v", err)
	}
	return
}
