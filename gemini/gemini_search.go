package gemini

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// GeminiAPIResponse represents the top-level structure of the Gemini API's content generation response.
type GeminiAPIResponse struct {
	Candidates []Candidate `json:"candidates"`
}

// Candidate represents a single generated response candidate from the model.
type Candidate struct {
	Content           Content           `json:"content"`
	GroundingMetadata GroundingMetadata `json:"groundingMetadata"`
}

// Content represents the actual generated text and its role.
type Content struct {
	Parts []Part `json:"parts"`
	Role  string `json:"role"` // "model" or "user"
}

// Part represents a segment of the content, primarily text in this case.
type Part struct {
	Text string `json:"text"`
	// Other possible parts like ImageData would go here if you handle multimodal responses
}

// GroundingMetadata provides information about the sources used for grounding the response.
type GroundingMetadata struct {
	WebSearchQueries  []string           `json:"webSearchQueries"`
	SearchEntryPoint  SearchEntryPoint   `json:"searchEntryPoint"`
	GroundingChunks   []GroundingChunk   `json:"groundingChunks"`
	GroundingSupports []GroundingSupport `json:"groundingSupports"`
}

// SearchEntryPoint provides information about the search UI component.
type SearchEntryPoint struct {
	RenderedContent string `json:"renderedContent"`
}

// GroundingChunk represents a piece of source material (e.g., a web page).
type GroundingChunk struct {
	Web struct {
		URI   string `json:"uri"`
		Title string `json:"title"`
	} `json:"web"`
}

// GroundingSupport maps a segment of the generated text to its supporting chunks.
type GroundingSupport struct {
	Segment               Segment `json:"segment"`
	GroundingChunkIndices []int   `json:"groundingChunkIndices"`
}

// Segment defines a portion of the text by start and end index.
type Segment struct {
	StartIndex int    `json:"startIndex"`
	EndIndex   int    `json:"endIndex"`
	Text       string `json:"text"`
}

func GenerateByGeminiWithSearch(userMsg string, location string) (string, error) {
	// 從環境變數獲取 GEMINI_API_KEY
	geminiAPIKey := os.Getenv("GEMINI_API_KEY")
	if geminiAPIKey == "" {
		return "", fmt.Errorf("GEMINI_API_KEY not set in environment variables")
	}

	// 定義 API 端點
	url := "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent?key=" + geminiAPIKey

	// 定義請求的內容 (request body)
	requestBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]string{
					{"text": fmt.Sprintf("%s\nLocation: %s", userMsg, location)},
				},
			},
		},
		"tools": []map[string]interface{}{
			{
				"google_search": map[string]interface{}{}, // 啟用 Google Search 工具
			},
		},
	}

	// 將請求內容編碼為 JSON
	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return "", err
	}

	// 創建一個新的 HTTP 請求
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}

	// 設定 Content-Type header
	req.Header.Set("Content-Type", "application/json")

	// 發送請求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close() // 確保在函數結束時關閉響應體

	// 讀取響應內容
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// 打印響應狀態碼和內容
	fmt.Printf("Response Status: %s\n", resp.Status)
	fmt.Printf("Response Body: %s\n", responseBody)

	var response GeminiAPIResponse
	err = json.Unmarshal(responseBody, &response)
	if err != nil {
		return "", err
	}

	if len(response.Candidates) > 0 {
		candidate := response.Candidates[0]
		result := ""
		modelResp := ""

		if len(candidate.Content.Parts) > 0 {
			modelResp = candidate.Content.Parts[0].Text
		}
		chunkResp := ""
		// 檢查是否有 grounding metadata
		if len(candidate.GroundingMetadata.GroundingChunks) > 0 {
			for idx, chunk := range candidate.GroundingMetadata.GroundingChunks {
				chunkResp += fmt.Sprintf("Source[%d][%s]: %s\n", idx, chunk.Web.Title, chunk.Web.URI)
			}
		}

		result = fmt.Sprintf("%s\n[Sources]\n%s", modelResp, chunkResp)

		// 返回第一個候選者的文本內容
		return result, nil
	}
	return "", fmt.Errorf("no candidates found in response")
}
