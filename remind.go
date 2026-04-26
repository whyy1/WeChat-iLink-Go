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

// ListReminders returns all pending (future) reminders for a user.
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

	return s.removeLocked(userID, id)
}

func (s *ReminderStore) removeLocked(userID, id string) bool {
	list := s.reminders[userID]
	for i, r := range list {
		if r.ID == id {
			s.reminders[userID] = append(list[:i], list[i+1:]...)
			return true
		}
	}
	return false
}

// Start launches the background dispatcher. It checks for due reminders every second
// and sends them via the provided client. Only successfully dispatched reminders are removed.
// Blocks until ctx is cancelled.
func (s *ReminderStore) Start(ctx context.Context, client *Client) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.dispatch(client)
		}
	}
}

func (s *ReminderStore) dispatch(client *Client) {
	s.mu.Lock()
	due := s.peekDueLocked()
	s.mu.Unlock()

	for _, r := range due {
		msg := fmt.Sprintf("⏰ 提醒：%s", r.Message)
		err := client.SendTextSimple(r.UserID, r.ContextToken, msg)
		if err != nil {
			log.Printf("reminder dispatch failed user=%s id=%s: %v (will retry)", r.UserID, r.ID, err)
			continue
		}
		s.mu.Lock()
		s.removeLocked(r.UserID, r.ID)
		s.mu.Unlock()
	}
}

// peekDueLocked returns due reminders without removing them. Caller must hold s.mu.
func (s *ReminderStore) peekDueLocked() []Reminder {
	now := time.Now()
	var due []Reminder
	for _, list := range s.reminders {
		for _, r := range list {
			if !r.TriggerAt.After(now) {
				due = append(due, r)
			}
		}
	}
	return due
}
