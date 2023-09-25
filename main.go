package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
)

// QueryResult struct represents the JSON structure of the API response
type QueryResult struct {
	QueryResult struct {
		Data struct {
			Rows []interface{} `json:"rows"`
		} `json:"data"`
	} `json:"query_result"`
}

type DataProcessor func([]interface{}) (interface{}, error)

// init function to load environment variables from a .env file
func init() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}
}

func main() {
	redashBaseURL := getEnv("REDASH_BASE_URL", "")
	redashAPIKey := getEnv("REDASH_API_KEY", "")
	redashQueryID := getEnv("REDASH_QUERY_ID", "")
	googleChatWebhookURL := getEnv("GOOGLE_CHAT_WEBHOOK_URL", "")

	// Construct URLs
	refreshURL := fmt.Sprintf("%s/api/queries/%s/refresh", redashBaseURL, redashQueryID)
	resultsURL := fmt.Sprintf("%s/api/queries/%s/results.json?api_key=%s", redashBaseURL, redashQueryID, redashAPIKey)

	// Create a custom HTTP client
	client := &http.Client{}

	// Refresh the query with Authorization header
	req, err := http.NewRequest("POST", refreshURL, nil)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Add("Authorization", fmt.Sprintf("Key %s", redashAPIKey))

	_, err = client.Do(req)
	if err != nil {
		log.Fatal(err)
	}

	// Sleep or poll until the results are ready
	time.Sleep(10 * time.Second) // Adjust as needed

	// Fetch the results using API key in the URL
	resp, err := http.Get(resultsURL)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	var result QueryResult
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		log.Fatal(err)
	}

	var processor DataProcessor = countMembersProcessor
	processedData, err := processor(result.QueryResult.Data.Rows)
	if err != nil {
		log.Printf("Error processing data: %v", err)
		return
	}

	count, ok := processedData.(int)
	if !ok {
		log.Printf("Error: Processed data is not an integer")
		return
	}

	err = sendMessageToGoogleChat(googleChatWebhookURL, count)
	if err != nil {
		log.Printf("Error sending message to Google Chat: %v", err)
	}
}

// countMembersProcessor returns the number of rows as the processed data
func countMembersProcessor(rows []interface{}) (interface{}, error) {
	return len(rows), nil
}

// sendMessageToGoogleChat sends the count message to Google Chat Webhook
func sendMessageToGoogleChat(webhookURL string, count int) error {
	message := map[string]interface{}{
		"text": fmt.Sprintf("Total Member Counts: %d", count),
	}

	messageBytes, err := json.Marshal(message)
	if err != nil {
		return err
	}

	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(messageBytes))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to send message to Google Chat, status: %d, response: %s", resp.StatusCode, string(body))
	}

	return nil
}

// getEnv fetches the value of the environment variable identified by key,
// returns fallback if the environment variable is not set.
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
