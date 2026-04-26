package ilink

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// Reminder represents a scheduled reminder for a user.
type Reminder struct {
	ID           string
	UserID       string
	ContextToken string
	Message      string
	TriggerAt    time.Time
}

// ReminderStore manages reminders in memory and dispatches them when due.
type ReminderStore struct {
	mu        sync.Mutex
	reminders map[string][]Reminder
	counter   int64
}

// NewReminderStore creates an empty reminder store.
func NewReminderStore() *ReminderStore {
	return &ReminderStore{
		reminders: make(map[string][]Reminder),
	}
}

// AddReminder schedules a new reminder. Returns the reminder ID.
func (s *ReminderStore) AddReminder(userID, contextToken, message string, triggerAt time.Time) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.counter++
	id := fmt.Sprintf("r_%d_%d", time.Now().UnixNano(), s.counter)
	r := Reminder{
		ID:           id,
		UserID:       userID,
		ContextToken: contextToken,
		Message:      message,
		TriggerAt:    triggerAt,
	}
	s.reminders[userID] = append(s.reminders[userID], r)
	return id
}

// ListReminders returns all pending reminders for a user.
func (s *ReminderStore) ListReminders(userID string) []Reminder {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]Reminder, 0, len(s.reminders[userID]))
	for _, r := range s.reminders[userID] {
		if r.TriggerAt.After(time.Now()) {
			result = append(result, r)
		}
	}
	return result
}

// RemoveReminder removes a reminder by ID.
func (s *ReminderStore) RemoveReminder(userID, id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	list := s.reminders[userID]
	for i, r := range list {
		if r.ID == id {
			s.reminders[userID] = append(list[:i], list[i+1:]...)
			return true
		}
	}
	return false
}

// dueReminders returns reminders that are due and removes them from the store.
func (s *ReminderStore) dueReminders() []Reminder {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	var due []Reminder
	for userID, list := range s.reminders {
		var remaining []Reminder
		for _, r := range list {
			if !r.TriggerAt.After(now) {
				due = append(due, r)
			} else {
				remaining = append(remaining, r)
			}
		}
		s.reminders[userID] = remaining
	}
	return due
}

// Start launches the background dispatcher. It checks for due reminders every second
// and sends them via the provided client. Blocks until ctx is cancelled.
func (s *ReminderStore) Start(ctx context.Context, client *Client) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.dispatch(ctx, client)
		}
	}
}

func (s *ReminderStore) dispatch(ctx context.Context, client *Client) {
	due := s.dueReminders()
	for _, r := range due {
		msg := fmt.Sprintf("⏰ 提醒：%s", r.Message)
		err := client.SendTextSimple(r.UserID, r.ContextToken, msg)
		if err != nil {
			log.Printf("reminder dispatch failed user=%s id=%s: %v", r.UserID, r.ID, err)
		}
	}
}
