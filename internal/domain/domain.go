package domain

import (
	"time"
)

type User struct {
	ID        int64    	`json:"id" db:"id"`
	Name	  string	`json:"name" db:"name"`
}

type Expense struct {
	ID        int64     		`json:"id" db:"id"`
	UserID    int64     		`json:"user_id" db:"user_id"`
	Amount    float64   		`json:"amount" db:"amount"`
	Category  string			`json:"category" db:"category"`
	SpentAt   time.Time 		`json:"spent_at" db:"spent_at"`
	Comment   *string   		`json:"comment" db:"comment"`
	CreatedAt time.Time 		`json:"created_at" db:"created_at"`
	UpdatedAt time.Time 		`json:"updated_at" db:"updated_at"`
}

type ResponseType string

const (
	Summary			ResponseType = "summary"
	List			ResponseType = "list"
	Confirmation	ResponseType = "confirmation"
)

type ExpenseFilter struct {
	UserID    *int64
	Category  *string
	StartDate *time.Time
	EndDate   *time.Time
	Comment	  *string
}

type ExpenseAction struct {
	Action       string        `json:"action"`
	ResponseType ResponseType  `json:"response_type,omitempty"`
	
	Amount       float64       `json:"amount,omitempty"`
	Category     string        `json:"category,omitempty"`
	DateStr      string        `json:"date,omitempty"`
	StartDateStr string        `json:"start_date,omitempty"`
	EndDateStr   string        `json:"end_date,omitempty"`
	Comment      string        `json:"comment,omitempty"`
	
	NewAmount    float64       `json:"new_amount,omitempty"`
	NewCategory  string        `json:"new_category,omitempty"`
	NewDateStr   string        `json:"new_date,omitempty"`
	NewComment   string        `json:"new_comment,omitempty"`
}
