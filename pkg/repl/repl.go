package repl

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/chzyer/readline"
	"github.com/fatih/color"
	"github.com/nickhildpac/cli-gocurl/pkg/display"
	"github.com/nickhildpac/cli-gocurl/pkg/executor"
	"github.com/nickhildpac/cli-gocurl/pkg/parser"
)

var commandHistory []string

// Start initializes and runs the REPL.
func Start() {
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          color.CyanString("gocurl> "),
		HistoryFile:     "/tmp/gocurl-history.tmp",
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		panic(err)
	}
	defer rl.Close()

	display.PrintSuccess("Welcome to go-curl! Type /help for a list of commands.")

	for {
		line, err := rl.Readline()
		if err == readline.ErrInterrupt || err == io.EOF {
			break
		}
		if err != nil {
			display.PrintError("Error reading line: %v", err)
			continue
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		commandHistory = append(commandHistory, line)

		parts, err := parser.ParseCommand(line)
		if err != nil {
			display.PrintError("Error parsing command: %v", err)
			continue
		}

		command := parts[0]
		args := parts[1:]

		switch strings.ToUpper(command) {
		case "/GET":
			handleRequest("GET", args)
		case "/POST":
			handleRequest("POST", args)
		case "/PUT":
			handleRequest("PUT", args)
		case "/DELETE":
			handleRequest("DELETE", args)
		case "/HELP":
			printHelp()
		case "/HISTORY":
			printHistory()
		case "/CLEAR":
			display.ClearScreen()
		case "/EXIT", "/QUIT":
			return
		default:
			display.PrintWarning("Unknown command: %s. Type /help for available commands.", command)
		}
	}
}

func handleRequest(method string, args []string) {
	fs := flag.NewFlagSet(method, flag.ContinueOnError)
	var headers []string
	fs.Func("H", "Custom header (repeatable)", func(h string) error {
		headers = append(headers, h)
		return nil
	})
	data := fs.String("d", "", "Request body for POST/PUT")
	verbose := fs.Bool("v", false, "Verbose output")
	outputFile := fs.String("o", "", "Save response to file")

	if len(args) == 0 {
		display.PrintError("Usage: /%s <url> [flags]", method)
		return
	}
	url := args[0]
	fs.Parse(args[1:])

	curlArgs := []string{"curl", "-X", method, url}
	for _, h := range headers {
		curlArgs = append(curlArgs, "-H", h)
	}
	if (method == "POST" || method == "PUT") && *data != "" {
		curlArgs = append(curlArgs, "-d", *data)
	}
	if *verbose {
		curlArgs = append(curlArgs, "-v")
	}
	if *outputFile != "" {
		curlArgs = append(curlArgs, "-o", *outputFile)
	}

	executeCurl(curlArgs)
}

func executeCurl(curlArgs []string) {
	display.PrintInfo("Executing: %s", strings.Join(curlArgs, " "))
	output, err := executor.ExecuteAndCapture(curlArgs)
	if err != nil {
		// The error from curl already includes detailed output, so we just print that.
	}
	display.PrintOutput(output)
}

func printHelp() {
	display.PrintSuccess("Available commands:")
	fmt.Println("  /GET, /POST, /PUT, /DELETE <url> [flags]")
	fmt.Println("  /help                - Show this help message")
	fmt.Println("  /history             - Show command history for this session")
	fmt.Println("  /clear               - Clear the screen")
	fmt.Println("  /exit, /quit         - Exit the application")
	fmt.Println("\nFlags:")
	fmt.Println("  -H <header>          - Custom header (e.g., 'Content-Type: application/json'). Can be used multiple times.")
	fmt.Println("  -d <data>            - Request body for POST and PUT requests.")
	fmt.Println("  -v                   - Verbose output from curl.")
	fmt.Println("  -o <file>            - Save response to a file.")
}

func printHistory() {
	display.PrintSuccess("Command History:")
	for i, cmd := range commandHistory {
		fmt.Printf("  %d: %s\n", i+1, cmd)
	}
}
