package ilink

import (
	"testing"
	"time"
)

func TestReminderStoreAdd(t *testing.T) {
	s := NewReminderStore()
	triggerAt := time.Now().Add(5 * time.Minute)

	id := s.AddReminder("user1", "ctx1", "开会", triggerAt)
	if id == "" {
		t.Fatal("reminder ID should not be empty")
	}

	reminders := s.ListReminders("user1")
	if len(reminders) != 1 {
		t.Fatalf("expected 1 reminder, got %d", len(reminders))
	}
	if reminders[0].Message != "开会" {
		t.Fatalf("message: got %q", reminders[0].Message)
	}
	if reminders[0].ContextToken != "ctx1" {
		t.Fatalf("context_token: got %q", reminders[0].ContextToken)
	}
}

func TestReminderStoreMultipleUsers(t *testing.T) {
	s := NewReminderStore()
	triggerAt := time.Now().Add(5 * time.Minute)

	s.AddReminder("user1", "ctx1", "提醒1", triggerAt)
	s.AddReminder("user2", "ctx2", "提醒2", triggerAt)
	s.AddReminder("user1", "ctx1", "提醒3", triggerAt)

	if len(s.ListReminders("user1")) != 2 {
		t.Fatalf("user1: expected 2, got %d", len(s.ListReminders("user1")))
	}
	if len(s.ListReminders("user2")) != 1 {
		t.Fatalf("user2: expected 1, got %d", len(s.ListReminders("user2")))
	}
}

func TestReminderStoreRemove(t *testing.T) {
	s := NewReminderStore()
	triggerAt := time.Now().Add(5 * time.Minute)

	id := s.AddReminder("user1", "ctx1", "开会", triggerAt)

	ok := s.RemoveReminder("user1", id)
	if !ok {
		t.Fatal("should find and remove the reminder")
	}
	if len(s.ListReminders("user1")) != 0 {
		t.Fatal("reminder should be removed")
	}
	ok = s.RemoveReminder("user1", id)
	if ok {
		t.Fatal("should not find already-removed reminder")
	}
}

func TestReminderStoreListOnlyFuture(t *testing.T) {
	s := NewReminderStore()
	s.AddReminder("user1", "ctx1", "已过期", time.Now().Add(-1*time.Minute))
	s.AddReminder("user1", "ctx1", "未来提醒", time.Now().Add(5*time.Minute))

	reminders := s.ListReminders("user1")
	if len(reminders) != 1 {
		t.Fatalf("expected 1 future reminder, got %d", len(reminders))
	}
	if reminders[0].Message != "未来提醒" {
		t.Fatalf("message: got %q", reminders[0].Message)
	}
}

func TestReminderStoreEmpty(t *testing.T) {
	s := NewReminderStore()
	if len(s.ListReminders("nobody")) != 0 {
		t.Fatal("should return empty for unknown user")
	}
}
