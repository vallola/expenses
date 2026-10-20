package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/vallola/expenses/internal/config"
	"github.com/vallola/expenses/internal/domain"
	"go.uber.org/zap"
)

type AIAgent struct {
	apiKey 		string
	modelName	string
	log			*zap.Logger
}

func NewAIAgent(cfg *config.Config, log *zap.Logger) *AIAgent {
	return &AIAgent{
		apiKey: cfg.ApiKey,
		modelName: cfg.ModelName,
		log: log,
	}
}

func (a *AIAgent) ParseMessage(ctx context.Context, userMessage string) (*domain.ExpenseAction, error) {
	now := time.Now()

	prompt := fmt.Sprintf(
		"Current date and time: %s.\n"+
			"You are an AI assistant for a family expense tracking bot. Extract the user's intent into the required JSON format.\n\n"+
			"CRITICAL RULES FOR DATES:\n"+
			"1. For 'create' actions, populate `date` strictly in `YYYY-MM-DD` format (e.g., '2026-10-17'). If the user says 'вчера', calculate yesterday's date.\n"+
			"2. For 'list', 'summary', 'update', or 'delete' actions involving timeframes, calculate and populate BOTH `start_date` and `end_date` STRICTLY in `YYYY-MM-DD` format (e.g., '2026-07-01' to '2026-07-31').\n"+
			"3. NEVER include time, timezone letters (like T or Z), or extra commentary in any date field. Only output clean YYYY-MM-DD strings.\n"+
			"4. For historical ranges (e.g., 'три месяца назад', 'в июле'), calculate the exact first and last day of that period.\n\n"+
			"CRITICAL RULES FOR UPDATES & DELETES:\n"+
			"1. Use search fields (`category`, `amount`, `comment`, `start_date`, `end_date`) to define filters for finding the target record.\n"+
			"2. For 'update' actions, place the *new* intended values into `new_amount`, `new_category`, `new_date`, or `new_comment`. Leave them blank if they are not changing.\n\n"+
			"STRICT CATEGORY TAXONOMY & EXAMPLES:\n"+
			"- еда: Groceries, supermarkets, restaurants, cafes, bars, food delivery, coffee, snacks.\n"+
			"- транспорт: Taxis, public transit, carsharing, fuel, parking, tickets.\n"+
			"- жильё: Rent, mortgage, utilities, household items, home repairs.\n"+
			"- развлечения: Subscriptions, cinema, games, hobbies, events.\n"+
			"- здоровье: Pharmacy, medicines, doctor appointments, gym, fitness.\n"+
			"- покупки: Marketplaces, clothing, shoes, electronics, cosmetics, gifts.\n"+
			"- другое: Anything else (fines, bank fees, cash transfers).\n\n"+
			"EXAMPLES:\n"+
			"- Input: 'сколько потратил сегодня на еду'\n"+
			"  Output: {\"action\": \"list\", \"response_type\": \"summary\", \"category\": \"еда\", \"start_date\": \"%s\", \"end_date\": \"%s\"}\n"+
			"- Input: 'вчера купил кофе за 300'\n"+
			"  Output: {\"action\": \"create\", \"response_type\": \"confirmation\", \"amount\": 300, \"category\": \"еда\", \"date\": \"%s\", \"comment\": \"кофе\"}\n"+
			"- Input: 'измени вчерашний кофе на 350'\n"+
			"  Output: {\"action\": \"update\", \"response_type\": \"confirmation\", \"category\": \"еда\", \"comment\": \"кофе\", \"new_amount\": 350, \"start_date\": \"%s\", \"end_date\": \"%s\"}\n"+
			"- Input: 'удали трату на такси вчера'\n"+
			"  Output: {\"action\": \"delete\", \"response_type\": \"confirmation\", \"category\": \"транспорт\", \"start_date\": \"%s\", \"end_date\": \"%s\", \"comment\": \"такси\"}\n\n"+
			"User message to process: %s",
		now.Format(time.RFC1123),
		now.Format("2006-01-02"),                               // today for start_date
		now.Format("2006-01-02"),                               // today for end_date
		now.AddDate(0, 0, -1).Format("2006-01-02"),             // yesterday for create date
		now.AddDate(0, 0, -1).Format("2006-01-02"),             // yesterday start for update example
		now.AddDate(0, 0, -1).Format("2006-01-02"),             // yesterday end for update example
		now.AddDate(0, 0, -1).Format("2006-01-02"),             // yesterday start for delete example
		now.AddDate(0, 0, -1).Format("2006-01-02"),             // yesterday end for delete example
		userMessage,
	)

	payload := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]string{
					{"text": prompt},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"responseMimeType": "application/json",
			"responseSchema": map[string]interface{}{
				"type": "OBJECT",
				"properties": map[string]interface{}{
					"action": map[string]interface{}{
						"type": "STRING",
						"enum": []string{"create", "list", "update", "delete"},
					},
					"response_type": map[string]interface{}{
						"type": "STRING",
						"enum": []string{"summary", "list", "confirmation"},
					},
					// Search / Filter Fields
					"amount": map[string]interface{}{
						"type":        "NUMBER",
						"description": "Amount to filter by when finding a record to update or delete.",
					},
					"category": map[string]interface{}{
						"type": "STRING",
						"enum": []string{"еда", "транспорт", "жильё", "развлечения", "здоровье", "покупки", "другое"},
					},
					"date": map[string]interface{}{
						"type":        "STRING",
						"description": "Specific date for a create action in YYYY-MM-DD format.",
					},
					"start_date": map[string]interface{}{
						"type":        "STRING",
						"description": "Start date for filtering ranges strictly in YYYY-MM-DD format. No time or timezone.",
					},
					"end_date": map[string]interface{}{
						"type":        "STRING",
						"description": "End date for filtering ranges strictly in YYYY-MM-DD format. No time or timezone.",
					},
					"comment": map[string]interface{}{
						"type":        "STRING",
						"description": "Comment keyword to filter by or match an existing record.",
					},
					// New Target Values (Used exclusively for 'update' actions)
					"new_amount": map[string]interface{}{
						"type":        "NUMBER",
						"description": "New amount value if updating an existing record.",
					},
					"new_category": map[string]interface{}{
						"type":        "STRING",
						"enum":        []string{"еда", "транспорт", "жильё", "развлечения", "здоровье", "покупки", "другое"},
						"description": "New category if reclassifying an existing record.",
					},
					"new_date": map[string]interface{}{
						"type":        "STRING",
						"description": "New date for the expense if moving it to another day, strictly in YYYY-MM-DD format.",
					},
					"new_comment": map[string]interface{}{
						"type":        "STRING",
						"description": "New comment text if updating the comment on an existing record.",
					},
				},
				"required": []string{"action", "response_type"},
			},
		},
	}

	bodyBytes, err := json.Marshal(payload)
	
	if err != nil {
		a.log.Error("Failed to marshal payload", zap.Error(err))
		return nil, err
	}

	// Native endpoint URL
	apiURL := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", a.modelName, a.apiKey)

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(bodyBytes))
	
	if err != nil {
		a.log.Error("Failed to create a post request", zap.Error(err))
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)

	if err != nil {
		a.log.Error("Failed to perform the request", zap.Error(err))
		return nil, err
	}
	defer resp.Body.Close()

	respBodyBytes, err := io.ReadAll(resp.Body)

	if err != nil {
		a.log.Error("Failed to read body bytes", zap.Error(err))
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		a.log.Error("AI request failed", zap.String("status", resp.Status), zap.String("body", string(respBodyBytes)))
		return nil, fmt.Errorf("AI api returned non-200 status: %d", resp.StatusCode)
	}

	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.Unmarshal(respBodyBytes, &result); err != nil {
		a.log.Error("Failed to unmarshal body bytes", zap.Error(err))
		return nil, err
	}

	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		a.log.Error("AI response candidates are empty")
		return nil, fmt.Errorf("empty response candidates from model")
	}

	jsonText := result.Candidates[0].Content.Parts[0].Text

	var action domain.ExpenseAction
	if err := json.Unmarshal([]byte(jsonText), &action); err != nil {
		a.log.Error("AI response json parse error", zap.Error(err))
		return nil, err
	}

	return &action, nil
}
