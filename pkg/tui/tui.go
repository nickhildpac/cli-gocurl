package tui

import (
	"encoding/json"
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/nickhildpac/cli-gocurl/pkg/executor"
	"github.com/nickhildpac/cli-gocurl/pkg/parser"
)

type model struct {
	textInput        textinput.Model
	historyViewport  viewport.Model
	outputViewport   viewport.Model
	spinner          spinner.Model
	isLoading        bool
	history          []string
	suggestions      []string
	activeSuggestion int
	isSuggesting     bool
	requestHistory   []string
	historyIndex     int
	status           string
	width, height    int
}

type responseMsg struct {
	status string
	body   string
}

var (
	titleStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true).Padding(0, 1)
	sectionStyle    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240"))
	suggestionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	activeStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	// helpStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	commands = []string{"/GET", "/POST", "/PUT", "/DELETE", "/HISTORY", "/CLEAR", "/HELP", "/EXIT"}
	flags    = []string{"-H", "-d", "-v", "-o"}
)

func InitialModel() model {
	ti := textinput.New()
	ti.Placeholder = "Type a command..."
	ti.Focus()

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	historyVP := viewport.New(10, 5)
	historyVP.SetContent("Welcome to go-curl! Type /help for commands.")

	outputVP := viewport.New(10, 10)
	outputVP.SetContent("Response output will appear here.")

	return model{
		textInput:       ti,
		spinner:         s,
		historyViewport: historyVP,
		outputViewport:  outputVP,
		suggestions:     []string{},
		requestHistory:  []string{},
		historyIndex:    0,
		status:          "N/A",
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
				m.activeSuggestion = (m.activeSuggestion - 1 + len(m.suggestions)) % len(m.suggestions)
			} else if !m.isSuggesting && len(m.requestHistory) > 0 {
				if m.historyIndex > 0 {
					m.historyIndex--
				}
				m.textInput.SetValue(m.requestHistory[m.historyIndex])
				m.textInput.CursorEnd()
			}
		case tea.KeyDown:
			if m.isSuggesting && len(m.suggestions) > 0 {
				m.activeSuggestion = (m.activeSuggestion + 1) % len(m.suggestions)
			} else if !m.isSuggesting && len(m.requestHistory) > 0 {
				if m.historyIndex < len(m.requestHistory)-1 {
					m.historyIndex++
					m.textInput.SetValue(m.requestHistory[m.historyIndex])
				} else {
					m.historyIndex = len(m.requestHistory)
					m.textInput.SetValue("")
				}
				m.textInput.CursorEnd()
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
		m.width = msg.Width
		m.height = msg.Height

		// 40% for input, 60% for output
		inputSectionHeight := m.height * 2 / 5
		outputSectionHeight := m.height - inputSectionHeight - 2

		m.textInput.Width = m.width - 4

		// History viewport takes up the space in the input section not used by the text input itself
		m.historyViewport.Width = m.width - 4
		m.historyViewport.Height = inputSectionHeight - 4

		m.outputViewport.Width = m.width - 4
		m.outputViewport.Height = outputSectionHeight - 2

	case responseMsg:
		m.isLoading = false
		m.status = msg.status
		m.outputViewport.SetContent(formatJSON(msg.body))
		m.outputViewport.GotoTop()
		m.history = append(m.history, "✓ Request finished with status: "+msg.status)
		m.historyViewport.SetContent(strings.Join(m.history, "\n"))
		m.historyViewport.GotoBottom()

	case spinner.TickMsg:
		if m.isLoading {
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	m.textInput, cmd = m.textInput.Update(msg)
	cmds = append(cmds, cmd)
	m.historyViewport, cmd = m.historyViewport.Update(msg)
	cmds = append(cmds, cmd)
	m.outputViewport, cmd = m.outputViewport.Update(msg)
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

func (m *model) handleEnter() (tea.Model, tea.Cmd) {
	userInput := m.textInput.Value()
	m.history = append(m.history, "> "+userInput)
	m.historyViewport.SetContent(strings.Join(m.history, "\n"))
	m.textInput.Reset()
	m.historyViewport.GotoBottom()

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
		m.historyViewport.SetContent("")
		m.outputViewport.SetContent("")
		m.status = "N/A"
		return m, nil
	case "/HELP":
		m.history = append(m.history, getHelp())
		return m, nil
	case "/HISTORY":
		m.history = append(m.history, m.getUniqueHistory())
		return m, nil
	case "/GET", "/POST", "/PUT", "/DELETE":
		m.requestHistory = append(m.requestHistory, userInput)
		m.historyIndex = len(m.requestHistory)
		m.isLoading = true
		return m, tea.Batch(m.spinner.Tick, makeRequest(command[1:], args))
	default:
		m.history = append(m.history, "Unknown command: "+command)
		return m, nil
	}
}

func makeRequest(method string, args []string) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(100 * time.Millisecond)

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
			return responseMsg{status: "Error", body: "Usage: /" + method + " <url> [flags]"}
		}
		url := args[0]
		fs.Parse(args[1:])

		curlArgs := []string{"curl", "-i", "-X", method, url}
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
			return responseMsg{status: "Execution Error", body: fmt.Sprintf("%v\n%s", err, output)}
		}

		parts := strings.SplitN(output, "\r\n\r\n", 2)
		if len(parts) < 2 {
			parts = strings.SplitN(output, "\n\n", 2)
			if len(parts) < 2 {
				return responseMsg{status: "Parse Error", body: "Could not separate headers from body."}
			}
		}
		headerBlock := parts[0]
		body := parts[1]
		statusLine := strings.SplitN(headerBlock, "\n", 2)[0]

		return responseMsg{status: strings.TrimSpace(statusLine), body: body}
	}
}

func (m *model) getUniqueHistory() string {
	var uniqueHistory []string
	seen := make(map[string]bool)
	for i := len(m.requestHistory) - 1; i >= 0; i-- {
		cmd := m.requestHistory[i]
		if !seen[cmd] {
			seen[cmd] = true

			uniqueHistory = append(uniqueHistory, cmd)
		}
		if len(uniqueHistory) >= 10 {
			break
		}
	}
	if len(uniqueHistory) == 0 {
		return "No request history yet."
	}
	var b strings.Builder
	b.WriteString("Last 10 unique requests:\n")
	for i := len(uniqueHistory) - 1; i >= 0; i-- {
		b.WriteString(fmt.Sprintf("  %s\n", uniqueHistory[i]))
	}
	return b.String()
}

func (m model) View() string {
	// --- INPUT SECTION ---
	var inputContent strings.Builder
	if m.isLoading {
		inputContent.WriteString(m.spinner.View() + " Executing request...")
	} else {
		inputContent.WriteString(m.textInput.View())
	}

	if m.isSuggesting && len(m.suggestions) > 0 {
		inputContent.WriteString("\n")
		var suggestionParts []string
		for i, sug := range m.suggestions {
			if i == m.activeSuggestion {
				suggestionParts = append(suggestionParts, activeStyle.Render(sug))
			} else {
				suggestionParts = append(suggestionParts, suggestionStyle.Render(sug))
			}
		}
		inputContent.WriteString(lipgloss.JoinHorizontal(lipgloss.Left, suggestionParts...))
	}

	inputHistory := lipgloss.JoinVertical(lipgloss.Left,
		m.historyViewport.View(),
		inputContent.String(),
	)

	inputSection := sectionStyle.Copy().
		Width(m.width - 2).
		Height(m.height * 2 / 5).
		Render(titleStyle.Render("Input") + "\n" + inputHistory)

	// --- OUTPUT SECTION ---
	statusLine := fmt.Sprintf("Status: %s", m.status)
	outputContent := statusLine + "\n" + m.outputViewport.View()
	outputSection := sectionStyle.Copy().
		Width(m.width - 2).
		Height(m.height - lipgloss.Height(inputSection) - 2).
		Render(titleStyle.Render("Output") + "\n" + outputContent)

	return lipgloss.JoinVertical(lipgloss.Left,
		inputSection,
		outputSection,
	)
}

func getHelp() string {
	return `Available commands:
  /GET, /POST, /PUT, /DELETE <url> [flags]
  /history             - Show last 10 unique requests
  /help                - Show this help message
  /clear               - Clear all views
  /exit                - Exit the application`
}

func formatJSON(raw string) string {
	var parsed json.RawMessage
	err := json.Unmarshal([]byte(raw), &parsed)
	if err != nil {
		return raw
	}
	pretty, err := json.MarshalIndent(parsed, "", "  ")
	if err != nil {
		return raw
	}
	return string(pretty)
}
