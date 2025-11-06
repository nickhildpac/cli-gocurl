package tui

import (
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/nickhildpac/cli-gocurl/pkg/executor"
	"github.com/nickhildpac/cli-gocurl/pkg/parser"
)

// ... (styles and other definitions will go here)

type model struct {
	viewport         viewport.Model
	textInput        textinput.Model
	spinner          spinner.Model
	isLoading        bool
	history          []string
	suggestions      []string
	activeSuggestion int
	isSuggesting     bool
}

type responseMsg string

var (
	inputStyle        = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	suggestionStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	activeStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	viewportStyle     = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	helpStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	commands          = []string{"/GET", "/POST", "/PUT", "/DELETE", "/CLEAR", "/HELP", "/EXIT"}
	flags             = []string{"-", "-d", "-v", "-o"}
)

func InitialModel() model {
	ti := textinput.New()
	ti.Placeholder = "Type a command (e.g., /GET https://httpbin.org/get) or /help"
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 80

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	vp := viewport.New(80, 10)
	vp.SetContent("Welcome to go-curl!")

	return model{
		textInput:   ti,
		spinner:     s,
		viewport:    vp,
		suggestions: []string{},
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			if m.isSuggesting {
				m.isSuggesting = false
			} else {
				return m, tea.Quit
			}
		case tea.KeyUp:
			if m.isSuggesting && len(m.suggestions) > 0 {
				m.activeSuggestion--
				if m.activeSuggestion < 0 {
					m.activeSuggestion = len(m.suggestions) - 1
				}
			}
		case tea.KeyDown:
			if m.isSuggesting && len(m.suggestions) > 0 {
				m.activeSuggestion++
				if m.activeSuggestion >= len(m.suggestions) {
					m.activeSuggestion = 0
				}
			}
		case tea.KeyTab, tea.KeyEnter:
			if m.isSuggesting && len(m.suggestions) > 0 {
				parts := strings.Fields(m.textInput.Value())
				if len(parts) > 0 {
					lastPart := parts[len(parts)-1]
					if strings.HasPrefix(lastPart, "/") || strings.HasPrefix(lastPart, "-") {
						parts[len(parts)-1] = m.suggestions[m.activeSuggestion]
						m.textInput.SetValue(strings.Join(parts, " ") + " ")
					}
				}
				m.isSuggesting = false
				m.textInput.CursorEnd()
			} else if msg.Type == tea.KeyEnter {
				return m.handleEnter()
			}
		default:
			m.textInput, cmd = m.textInput.Update(msg)
			cmds = append(cmds, cmd)
			m.updateSuggestions()
			return m, tea.Batch(cmds...)
		}

	case tea.WindowSizeMsg:
		m.textInput.Width = msg.Width - 2
		m.viewport.Width = msg.Width - 2
		m.viewport.Height = msg.Height - 5

	case responseMsg:
		m.isLoading = false
		m.history = append(m.history, string(msg))
		m.viewport.SetContent(strings.Join(m.history, "\n"))
		m.viewport.GotoBottom()

	case spinner.TickMsg:
		if m.isLoading {
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	m.textInput, cmd = m.textInput.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m *model) updateSuggestions() {
	val := m.textInput.Value()
	parts := strings.Fields(val)
	m.suggestions = []string{}

	if val == "" || val[len(val)-1] == ' ' {
		m.isSuggesting = false
		return
	}

	lastPart := parts[len(parts)-1]
	if strings.HasPrefix(lastPart, "/") {
		m.isSuggesting = true
		for _, cmd := range commands {
			if strings.HasPrefix(strings.ToUpper(cmd), strings.ToUpper(lastPart)) {
				m.suggestions = append(m.suggestions, cmd)
			}
		}
	} else if strings.HasPrefix(lastPart, "-") {
		m.isSuggesting = true
		for _, f := range flags {
			if strings.HasPrefix(f, lastPart) {
				m.suggestions = append(m.suggestions, f)
			}
		}
	} else {
		m.isSuggesting = false
	}
	m.activeSuggestion = 0
}

func (m model) handleEnter() (tea.Model, tea.Cmd) {
	userInput := m.textInput.Value()
	m.history = append(m.history, "> "+userInput)
	m.viewport.SetContent(strings.Join(m.history, "\n"))
	m.textInput.Reset()
	m.viewport.GotoBottom()

	parts, err := parser.ParseCommand(userInput)
	if err != nil {
		m.history = append(m.history, "Error: "+err.Error())
		return m, nil
	}

	command := strings.ToUpper(parts[0])
	args := parts[1:]

	switch command {
	case "/EXIT":
		return m, tea.Quit
	case "/CLEAR":
		m.history = []string{}
		m.viewport.SetContent("")
		return m, nil
	case "/HELP":
		m.history = append(m.history, getHelp())
		return m, nil
	case "/GET", "/POST", "/PUT", "/DELETE":
		m.isLoading = true
		return m, tea.Batch(m.spinner.Tick, makeRequest(command[1:], args))
	default:
		m.history = append(m.history, "Unknown command: "+command)
		return m, nil
	}
}

func makeRequest(method string, args []string) tea.Cmd {
	return func() tea.Msg {
		// A short delay to make the spinner visible
		time.Sleep(250 * time.Millisecond)

		fs := flag.NewFlagSet(method, flag.ContinueOnError)
		var headers []string
		fs.Func("H", "Custom header", func(h string) error {
			headers = append(headers, h)
			return nil
		})
		data := fs.String("d", "", "Request body")
		verbose := fs.Bool("v", false, "Verbose")
		outputFile := fs.String("o", "", "Output file")

		if len(args) == 0 {
			return responseMsg("Usage: /" + method + " <url> [flags]")
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

		output, err := executor.ExecuteAndCapture(curlArgs)
		if err != nil {
			return responseMsg(fmt.Sprintf("Error: %v\n%s", err, output))
		}
		return responseMsg(output)
	}
}

func getHelp() string {
	return `Available commands:
  /GET, /POST, /PUT, /DELETE <url> [flags]
  /help                - Show this help message
  /clear               - Clear the screen
  /exit                - Exit the application
Flags:
  -H <header>          - Custom header (e.g., 'Content-Type: application/json')
  -d <data>            - Request body for POST and PUT
  -v                   - Verbose output
  -o <file>            - Save response to a file`
}

func (m model) View() string {
	var s strings.Builder

	s.WriteString(viewportStyle.Render(m.viewport.View()))
	s.WriteString("\n")

	if m.isLoading {
		s.WriteString(fmt.Sprintf("%s Executing request...", m.spinner.View()))
	} else {
		s.WriteString(inputStyle.Render(m.textInput.View()))
	}

	if m.isSuggesting && len(m.suggestions) > 0 {
		s.WriteString("\n")
		for i, sug := range m.suggestions {
			if i == m.activeSuggestion {
				s.WriteString(activeStyle.Render(sug))
			} else {
				s.WriteString(suggestionStyle.Render(sug))
			}
			s.WriteString("  ")
		}
	} else {
		s.WriteString("\n" + helpStyle.Render("↑/↓ to navigate suggestions, <tab> to complete, <enter> to execute"))
	}

	return s.String()
}
