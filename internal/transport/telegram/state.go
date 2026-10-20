package telegram

import (
	"sync"
	"time"
)

type PendingActionType string

const (
	ActionUpdate PendingActionType = "update"
	ActionDelete PendingActionType = "delete"
)

type PendingAction struct {
	Type        PendingActionType
	ExpenseID   int64
	NewAmount   float64
	NewCategory string
	NewSpentAt  time.Time
	NewComment  *string
	ExpiresAt   time.Time
}

type StateManager struct {
	mu     sync.Mutex
	states map[string]PendingAction
}

func NewStateManager() *StateManager {
	sm := &StateManager{
		states: make(map[string]PendingAction),
	}
	go func() {
		for range time.Tick(1 * time.Minute) {
			sm.mu.Lock()
			now := time.Now()
			for k, v := range sm.states {
				if now.After(v.ExpiresAt) {
					delete(sm.states, k)
				}
			}
			sm.mu.Unlock()
		}
	}()
	return sm
}

func (sm *StateManager) Set(token string, action PendingAction) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	action.ExpiresAt = time.Now().Add(5 * time.Minute)
	sm.states[token] = action
}

func (sm *StateManager) Get(token string) (PendingAction, bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	act, ok := sm.states[token]
	if ok {
		delete(sm.states, token) // One-time use
	}
	return act, ok
}
