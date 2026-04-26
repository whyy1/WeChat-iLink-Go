package main

import (
	"context"
	"fmt"
	"os"
	"time"

	ilink "github.com/whyy1/WeChat-iLink-Go"
)

func main() {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_AUTH_TOKEN")
	}
	if apiKey == "" {
		fmt.Println("ERROR: no ANTHROPIC_API_KEY or ANTHROPIC_AUTH_TOKEN found")
		os.Exit(1)
	}

	baseURL := os.Getenv("ANTHROPIC_BASE_URL")
	model := os.Getenv("ANTHROPIC_DEFAULT_SONNET_MODEL")
	if model == "" {
		model = string(ilink.DefaultAgentModel)
	}

	fmt.Println("=== Claude Agent Integration Test ===")
	fmt.Printf("Model: %s\n", model)
	fmt.Printf("Base URL: %s\n", baseURL)

	reminderStore := ilink.NewReminderStore()

	agent := ilink.NewAgent(ilink.AgentConfig{
		APIKey:         apiKey,
		BaseURL:        baseURL,
		Model:          model,
		EnableCommands: true,
	})
	agent.SetReminderStore(reminderStore)

	// Test 1: Simple conversation
	fmt.Println("\n--- Test 1: Simple conversation ---")
	ctx1, cancel1 := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel1()

	reply1, err := agent.ChatWithCtx(ctx1, "test-user", "fake-ctx-token", "你好，请用一句话介绍自己。")
	if err != nil {
		fmt.Printf("FAILED: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Claude reply: %s\n", reply1)

	// Test 2: Tool use (get_current_time)
	fmt.Println("\n--- Test 2: Tool use (get_current_time) ---")
	ctx2, cancel2 := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel2()

	reply2, err := agent.ChatWithCtx(ctx2, "test-user", "fake-ctx-token", "现在几点了？")
	if err != nil {
		fmt.Printf("FAILED: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Claude reply: %s\n", reply2)

	// Test 3: Command execution
	fmt.Println("\n--- Test 3: Tool use (execute_command) ---")
	ctx3, cancel3 := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel3()

	reply3, err := agent.ChatWithCtx(ctx3, "test-user", "fake-ctx-token", "运行 echo hello_agent 并告诉我结果")
	if err != nil {
		fmt.Printf("FAILED: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Claude reply: %s\n", reply3)

	// Test 4: Set reminder
	fmt.Println("\n--- Test 4: Tool use (set_reminder) ---")
	ctx4, cancel4 := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel4()

	reply4, err := agent.ChatWithCtx(ctx4, "test-user", "fake-ctx-token", "1分钟后提醒我喝水")
	if err != nil {
		fmt.Printf("FAILED: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Claude reply: %s\n", reply4)

	// Verify reminder was stored
	reminders := reminderStore.ListReminders("test-user")
	if len(reminders) == 0 {
		fmt.Println("FAILED: no reminders stored")
		os.Exit(1)
	}
	fmt.Printf("Reminder stored: %+v\n", reminders[0])

	// Test 5: Conversation continuity
	fmt.Println("\n--- Test 5: Conversation continuity ---")
	ctx5, cancel5 := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel5()

	reply5, err := agent.ChatWithCtx(ctx5, "test-user", "fake-ctx-token", "我之前问了什么问题？请简要总结。")
	if err != nil {
		fmt.Printf("FAILED: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Claude reply: %s\n", reply5)

	fmt.Println("\n=== All tests passed! ===")
}
