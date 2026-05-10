package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	ilink "github.com/whyy1/WeChat-iLink-Go"
	"github.com/whyy1/WeChat-iLink-Go/internal/config"
)

func main() {
	// Handle "config" subcommand before flag parsing.
	if len(os.Args) > 1 && os.Args[1] == "config" {
		handleConfig(os.Args[2:])
		return
	}

	fs := flag.NewFlagSet("ilink", flag.ExitOnError)
	cmdMode := fs.Bool("cmd", false, "run in echo command mode (no Claude)")
	cmdModeShort := fs.Bool("c", false, "run in echo command mode (shorthand)")
	agentMode := fs.Bool("agent", false, "run in agent mode (default)")
	agentModeShort := fs.Bool("a", false, "run in agent mode (shorthand)")
	configPath := fs.String("config", "", "path to config file (default ~/.ilink/config.json)")

	fs.Parse(os.Args[1:])

	cfgPath := *configPath
	isCmd := *cmdMode || *cmdModeShort
	isAgent := *agentMode || *agentModeShort

	// Default to agent mode if neither flag is set.
	if !isCmd && !isAgent {
		isAgent = true
	}

	if isCmd {
		runCmdMode(cfgPath)
	} else {
		runAgentMode(cfgPath)
	}
}

func handleConfig(args []string) {
	// Extract --config flag from args manually.
	configPath := ""
	cleanArgs := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		if args[i] == "--config" && i+1 < len(args) {
			configPath = args[i+1]
			i++
		} else if len(args[i]) > 9 && args[i][:9] == "--config=" {
			configPath = args[i][9:]
		} else {
			cleanArgs = append(cleanArgs, args[i])
		}
	}

	subcommand := ""
	if len(cleanArgs) > 0 {
		subcommand = cleanArgs[0]
	}

	switch subcommand {
	case "", "setup":
		runInteractiveSetup(configPath)

	case "show":
		cfg := config.ResolveWithDefaults(configPath)
		fmt.Print(config.Show(cfg, configPath))

	case "set":
		if len(cleanArgs) < 3 {
			fmt.Println("Usage: ilink config set <key> <value>")
			fmt.Println("\nAvailable keys:")
			fmt.Println("  bot_token, ANTHROPIC_API_KEY, ANTHROPIC_BASE_URL, ANTHROPIC_MODEL,")
			fmt.Println("  backend, enable_commands, system_prompt, working_dir")
			os.Exit(1)
		}
		if err := config.Set(cleanArgs[1], cleanArgs[2], configPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Set %s = %s\n", cleanArgs[1], cleanArgs[2])

	default:
		fmt.Println("Usage: ilink config [setup|show|set]")
		fmt.Println("  setup          Interactive configuration wizard")
		fmt.Println("  show           Display current configuration")
		fmt.Println("  set <key> <val> Set a configuration value")
		os.Exit(1)
	}
}

func runInteractiveSetup(configPath string) {
	fmt.Println("=== iLink Bot Configuration ===")

	// Step 1: WeChat login.
	fmt.Println("\nStep 1: WeChat Login")
	loginClient := ilink.NewClient("")
	qrResp, err := loginClient.GetBotQRCodeSimple()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting QR code: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Please scan the QR code:")
	_ = printQRCode(qrResp.QRCodeURL)
	fmt.Println("Waiting for scan confirmation...")

	token, err := loginClient.WaitForLoginSimple(qrResp.QRCode, 2*time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Login failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Login successful!")

	// Step 2: Configure agent.
	fmt.Println("\nStep 2: Configure Agent")
	if _, err := config.InteractiveSetup(token, configPath); err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nConfiguration saved to: %s\n", config.FilePath(configPath))
	fmt.Println("\nSetup complete!")
	fmt.Println("  Run 'ilink' for agent mode")
	fmt.Println("  Run 'ilink --cmd' for echo mode")
}
