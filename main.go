package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/line/line-bot-sdk-go/v8/linebot"
	"github.com/line/line-bot-sdk-go/v8/linebot/messaging_api"
	"github.com/line/line-bot-sdk-go/v8/linebot/webhook"
	"github.com/patrickmn/go-cache"
	"google.golang.org/genai"
)

var c = cache.New(5*time.Minute, 10*time.Minute)
var imgCache = cache.New(180*time.Minute, 180*time.Minute)
var host = ""

// Request struct for the Grok API
type GrokRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

// Message struct for chat messages
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Response struct to parse Grok API response
type GrokResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
			Role    string `json:"role"`
		} `json:"message"`
	} `json:"choices"`
}

type ImageRequest struct {
	Prompt         string `json:"prompt"`
	N              int    `json:"n,omitempty"` // Number of images (1-10, default 1)
	Model          string `json:"model"`
	ResponseFormat string `json:"response_format,omitempty"` // Optional: response format (e.g., "url", "b64_json")
}

// ImageResponse defines the structure for the API response
type ImageResponse struct {
	Data []struct {
		URL string `json:"url"` // URL to the generated image
	} `json:"data"`
}

// Hypothetical Grok API function (replace with actual implementation if available)
func callGrokAPI(chatID string, message string) (string, error) {
	apiKey := os.Getenv("GROK_API_KEY")
	if apiKey == "" {
		log.Fatal("GROK_API_KEY not set in .env")
	}

	// Define the request payload
	request := GrokRequest{
		Model:    "grok-3-beta", // Use "grok-beta" or another available model
		Messages: []Message{},
	}
	// check have context in cache
	if context, found := c.Get(chatID); found {
		ctxMsgs := context.([]Message)

		// If context found, append it to the request
		request.Messages = append(request.Messages, ctxMsgs...)
	}

	thisUserMsg := Message{
		Role:    "user",
		Content: message,
	}
	// Append the user message to the request
	request.Messages = append(request.Messages, thisUserMsg)

	// Marshal the request to JSON
	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", err
	}

	// Create HTTP request
	url := "https://api.x.ai/v1/chat/completions"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Read and parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API request failed with status: %d", resp.StatusCode)
	}

	var grokResp GrokResponse
	err = json.Unmarshal(body, &grokResp)
	if err != nil {
		return "", fmt.Errorf("Error unmarshaling response: %v", err)
	}

	// Print the response
	result := "No choices returned in response"
	if len(grokResp.Choices) > 0 {
		result = grokResp.Choices[0].Message.Content
		request.Messages = append(request.Messages, Message{
			Role:    grokResp.Choices[0].Message.Role,
			Content: grokResp.Choices[0].Message.Content,
		})
	}

	// Store the context in cache
	if len(request.Messages) > 10 {
		request.Messages = request.Messages[len(request.Messages)-10:]
	}
	c.Set(chatID, request.Messages, cache.DefaultExpiration)

	return result, nil
}

// Hypothetical Grok API function (replace with actual implementation if available)
func getImgPromptByGrok(message string) (string, error) {
	apiKey := os.Getenv("GROK_API_KEY")
	if apiKey == "" {
		log.Fatal("GROK_API_KEY not set in .env")
	}

	// Define the request payload
	request := GrokRequest{
		Model:    "grok-3-beta", // Use "grok-beta" or another available model
		Messages: []Message{},
	}

	thisUserMsg := Message{
		Role:    "user",
		Content: fmt.Sprintf("you only output prompt for image generate. this is my msg:%s. only output prompt", message),
	}
	// Append the user message to the request
	request.Messages = append(request.Messages, thisUserMsg)

	// Marshal the request to JSON
	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", err
	}

	// Create HTTP request
	url := "https://api.x.ai/v1/chat/completions"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Read and parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API request failed with status: %d", resp.StatusCode)
	}

	var grokResp GrokResponse
	err = json.Unmarshal(body, &grokResp)
	if err != nil {
		return "", fmt.Errorf("Error unmarshaling response: %v", err)
	}

	// Print the response
	result := "No choices returned in response"
	if len(grokResp.Choices) > 0 {
		result = grokResp.Choices[0].Message.Content
	}
	log.Println("Grok img prompt:", result)
	return result, nil
}

func generateImageByGrok(userMsg string) (string, error) {
	prompt, err := getImgPromptByGrok(userMsg)
	if err != nil {
		log.Fatal(err)
	}

	apiKey := os.Getenv("GROK_API_KEY")
	if apiKey == "" {
		log.Fatal("GROK_API_KEY not set in .env")
	}
	url := "https://api.x.ai/v1/images/generations"

	// Create request payload
	reqBody := ImageRequest{
		Prompt:         prompt,
		N:              1, // Request 1 image; adjust as needed (max 10)
		Model:          "grok-2-image-1212",
		ResponseFormat: "url",
	}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("error marshaling request: %v", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("error creating request: %v", err)
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API request failed: %d, %s", resp.StatusCode, string(body))
	}

	// Parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %v", err)
	}

	var imgResp ImageResponse
	err = json.Unmarshal(body, &imgResp)
	if err != nil {
		return "", fmt.Errorf("error unmarshaling response: %v", err)
	}

	// Extract image URLs
	var imageURLs []string
	for _, item := range imgResp.Data {
		imageURLs = append(imageURLs, item.URL)
	}

	return imageURLs[0], nil
}

func generateImageByGemini(userMsg string) (string, error) {
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
		MaxOutputTokens:    &maxOutputTokens,
		ResponseModalities: []string{"IMAGE", "TEXT"},
	}
	// config.ResponseModalities = []string{"IMAGE", "TEXT"}
	// Call the GenerateContent method.

	promptMsg, err := getImgPromptByGrok(userMsg)
	if err != nil {
		log.Fatal(err)
	}

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

const maxLength = 4999

func splitString(input string) []string {
	var parts []string

	// If the string is already under maxLength, return it as a single part
	if len(input) <= maxLength {
		return []string{input}
	}

	// Split the string into chunks of maxLength
	start := 0
	for start < len(input) {
		end := start + maxLength
		if end > len(input) {
			end = len(input)
		}
		parts = append(parts, input[start:end])
		start = end
	}

	return parts
}

func main() {

	// Load environment variables
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// Initialize LINE bot client
	bot, err := messaging_api.NewMessagingApiAPI(
		os.Getenv("CHANNEL_TOKEN"),
	)
	if err != nil {
		log.Fatal(err)
	}
	endpointResp, err := bot.GetWebhookEndpoint()
	if err != nil {
		log.Fatal(err)
	}

	// only get host and scheme
	fullUrl, err := url.Parse(endpointResp.Endpoint)
	if err != nil {
		log.Fatal(err)
	}
	host = fullUrl.Scheme + "://" + fullUrl.Host
	// Webhook secret for signature validation
	channelSecret := os.Getenv("CHANNEL_SECRET")

	// Set up HTTP server
	http.HandleFunc("/img/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if imgData, found := imgCache.Get(id); found {
			w.Header().Set("Content-Type", "image/png")
			w.Write(imgData.([]byte))
		} else {
			http.NotFound(w, r)
		}
	})
	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		// Parse incoming LINE webhook request
		cb, err := webhook.ParseRequest(channelSecret, r)
		if err != nil {
			if err == linebot.ErrInvalidSignature {
				w.WriteHeader(400)
			} else {
				w.WriteHeader(500)
			}
			log.Printf("Error parsing request: %v", err)
			return
		}
		chatID := cb.Destination
		// Process each event
		for _, event := range cb.Events {
			switch e := event.(type) {
			case webhook.MessageEvent:
				switch msg := e.Message.(type) {
				case webhook.TextMessageContent:
					thisText := strings.TrimSpace(msg.Text)
					// Check if message starts with "AI@"
					if strings.HasPrefix(strings.ToLower(thisText), strings.ToLower("AI@")) {
						// Extract the message content after "AI@"
						grokMsg := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(thisText), strings.ToLower("AI@")))
						if grokMsg == "" {
							continue // Skip if no content after "AI@"
						}

						// Call Grok API
						response, err := callGrokAPI(chatID, grokMsg)
						if err != nil {
							log.Printf("Error calling Grok API: %v", err)
							response = "Sorry, I couldn't process your request."
						}

						responseSlices := splitString(response)

						replyMsg := []messaging_api.MessageInterface{}

						for _, responseM := range responseSlices {
							replyMsg = append(replyMsg, &messaging_api.TextMessage{
								Text: responseM,
							})
						}

						// Reply to the user via LINE
						_, err = bot.ReplyMessage(&messaging_api.ReplyMessageRequest{
							ReplyToken: e.ReplyToken,
							Messages:   replyMsg,
						})
						if err != nil {
							log.Printf("Error replying to message: %v", err)
						}
					} else if strings.HasPrefix(strings.ToLower(thisText), strings.ToLower("AI#")) {
						// Extract the message content after "AI@"
						grokMsg := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(thisText), strings.ToLower("AI#")))
						if grokMsg == "" {
							continue // Skip if no content after "AI@"
						}
						useGrok := false
						var response string
						if grokMsg[0] == '#' {
							grokMsg = grokMsg[1:] // remove the first character
							useGrok = true
						}
						fmt.Println("Use Grok:", useGrok)
						if useGrok {
							// Call Grok API
							response, err = generateImageByGrok(grokMsg)
							if err != nil {
								log.Printf("Error calling Grok API: %v", err)
								continue
							}
						} else {
							// Call Grok API
							response, err = generateImageByGemini(grokMsg)
							if err != nil {
								log.Printf("Error calling Gemini API: %v", err)
								continue
							}
						}

						// Reply to the user via LINE
						_, err = bot.ReplyMessage(&messaging_api.ReplyMessageRequest{
							ReplyToken: e.ReplyToken,
							Messages: []messaging_api.MessageInterface{
								&messaging_api.ImageMessage{
									OriginalContentUrl: response,
									PreviewImageUrl:    response,
								},
							},
						})
						if err != nil {
							log.Printf("Error replying to message: %v", err)
						}
					}
				}
			}
		}
	})

	// Start the server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Starting server on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
