// TODO: add cluster status
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"io/ioutil"
	"net/http"
	"os"
)

const configFile = "camunda_cli_config.json"

type Platform struct {
	Name         string `json:"name"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	OAuthURL     string `json:"oauth_url"`
	BaseURL      string `json:"base_url"`
	Audience     string `json:"audience"`
}

type Config struct {
	Platforms []Platform `json:"platforms"`
}

// ----------- CLUSTERS -----------

type Cluster struct {
	UUID       string `json:"uuid"`
	Name       string `json:"name"`
	Generation struct {
		Name string `json:"name"`
	} `json:"generation"`
}
type clustersMsg []Cluster

// ----------- MENU STATE ----------

type menuState int

const (
	stateMainMenu menuState = iota
	stateAddPlatform
	stateGetToken
	stateManageMenu
	stateEditPlatform
	stateDeletePlatform
	stateConfirmDelete
	stateShowToken
	stateListClusters
	stateQuit
)

type model struct {
	cfg           Config
	state         menuState
	cursor        int
	platformIndex int
	token         string
	errMsg        string
	textInputs    []textinput.Model
	focusIndex    int
	clusters      []Cluster
}

func initialModel() model {
	cfg := loadOrInitConfig()
	return model{
		cfg:           cfg,
		state:         stateMainMenu,
		cursor:        0,
		platformIndex: -1,
	}
}

func loadOrInitConfig() Config {
	var config Config
	if _, err := os.Stat(configFile); err == nil {
		data, _ := ioutil.ReadFile(configFile)
		json.Unmarshal(data, &config)
	}
	return config
}

func saveConfig(config Config) {
	data, _ := json.MarshalIndent(config, "", "  ")
	_ = ioutil.WriteFile(configFile, data, 0600)
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.state {

	// ======== MAIN MENU =======
	case stateMainMenu:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "ctrl+c", "q":
				m.state = stateQuit
				return m, tea.Quit
			case "up":
				if m.cursor > 0 {
					m.cursor--
				}
			case "down":
				if m.cursor < len(m.cfg.Platforms)+2 {
					m.cursor++
				}
			case "enter":
				switch m.cursor {
				case len(m.cfg.Platforms):
					// Add new
					m.platformIndex = -1
					m = m.initTextInputs()
					m.state = stateAddPlatform
					return m, nil
				case len(m.cfg.Platforms) + 1:
					// Manage
					m.cursor = 0
					m.state = stateManageMenu
				case len(m.cfg.Platforms) + 2:
					// Quit
					m.state = stateQuit
					return m, tea.Quit
				default:
					// Select a platform: get token
					m.platformIndex = m.cursor
					m.state = stateGetToken
					return m, func() tea.Msg {
						token, err := getAccessToken(m.cfg.Platforms[m.platformIndex])
						if err != nil {
							return errMsg(err.Error())
						}
						return tokenMsg(token)
					}
				}
			}
		}
		return m, nil

	// ======== GET TOKEN =======
	case stateGetToken:
		switch msg := msg.(type) {
		case tokenMsg:
			m.token = string(msg)
			m.state = stateShowToken
		case errMsg:
			m.errMsg = string(msg)
			m.state = stateShowToken
		}
		return m, nil

		// ======== SHOW TOKEN ======
	case stateShowToken:
		switch msg.(type) {
		case tea.KeyMsg:
			m.state = stateListClusters
			m.errMsg = ""
			m.clusters = nil
			return m, func() tea.Msg {
				clusters, err := fetchClusters(m.cfg.Platforms[m.platformIndex].BaseURL, m.token)
				if err != nil {
					return errMsg(err.Error())
				}
				return clustersMsg(clusters)
			}
		}
		return m, nil

	// ======== LIST CLUSTERS =======
	case stateListClusters:
		switch msg := msg.(type) {
		case clustersMsg:
			m.clusters = msg
			m.cursor = 0
		case errMsg:
			m.errMsg = string(msg)
		case tea.KeyMsg:
			switch msg.String() {
			case "esc", "q":
				m.state = stateMainMenu
				m.cursor = 0
			case "up":
				if m.cursor > 0 {
					m.cursor--
				}
			case "down":
				if m.cursor < len(m.clusters)-1 {
					m.cursor++
				}
			case "enter":
				// Action possible sur un cluster sélectionné
			}
		}
		return m, nil

	// ======== ADD PLATFORM ====
	case stateAddPlatform:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "esc":
				m.state = stateMainMenu
				m.cursor = 0
			case "tab", "shift+tab", "enter", "up", "down":
				s := msg.String()
				if s == "enter" && m.focusIndex == len(m.textInputs)-1 {
					// Save
					p := Platform{
						Name:         m.textInputs[0].Value(),
						ClientID:     m.textInputs[1].Value(),
						ClientSecret: m.textInputs[2].Value(),
						OAuthURL:     m.textInputs[3].Value(),
						BaseURL:      m.textInputs[4].Value(),
						Audience:     m.textInputs[5].Value(),
					}
					if m.platformIndex >= 0 && m.platformIndex < len(m.cfg.Platforms) {
						m.cfg.Platforms[m.platformIndex] = p
					} else {
						m.cfg.Platforms = append(m.cfg.Platforms, p)
					}
					saveConfig(m.cfg)
					m.state = stateMainMenu
					m.cursor = 0
					m.platformIndex = -1
					return m, nil
				}
				if s == "up" {
					m.focusIndex--
				} else {
					m.focusIndex++
				}
				if m.focusIndex >= len(m.textInputs) {
					m.focusIndex = 0
				} else if m.focusIndex < 0 {
					m.focusIndex = len(m.textInputs) - 1
				}
				for i := 0; i < len(m.textInputs); i++ {
					m.textInputs[i].Blur()
				}
				m.textInputs[m.focusIndex].Focus()
				return m, nil
			}
		}
		var cmds []tea.Cmd
		for i := range m.textInputs {
			var cmd tea.Cmd
			m.textInputs[i], cmd = m.textInputs[i].Update(msg)
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)

	// ======== MANAGE MENU =====
	case stateManageMenu:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "esc":
				m.state = stateMainMenu
				m.cursor = 0
			case "up":
				if m.cursor > 0 {
					m.cursor--
				}
			case "down":
				if m.cursor < len(m.cfg.Platforms) {
					m.cursor++
				}
			case "enter":
				if m.cursor == len(m.cfg.Platforms) {
					// Back
					m.state = stateMainMenu
					m.cursor = 0
				} else {
					// Edit/Delete menu
					m.platformIndex = m.cursor
					m.state = stateEditPlatform
					m.cursor = 0
				}
			}
		}
		return m, nil

	// ======== EDIT/DELETE PLATFORM MENU =====
	case stateEditPlatform:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "esc":
				m.state = stateManageMenu
				m.cursor = 0
			case "up":
				if m.cursor > 0 {
					m.cursor--
				}
			case "down":
				if m.cursor < 1 {
					m.cursor++
				}
			case "enter":
				if m.cursor == 0 {
					// Edit
					m = m.initEditInputs(m.cfg.Platforms[m.platformIndex])
					m.state = stateAddPlatform
					return m, nil
				} else if m.cursor == 1 {
					// Delete
					m.state = stateConfirmDelete
				}
			}
		}
		return m, nil

	// ======== CONFIRM DELETE =====
	case stateConfirmDelete:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "y", "Y":
				// Delete
				idx := m.platformIndex
				if idx >= 0 && idx < len(m.cfg.Platforms) {
					m.cfg.Platforms = append(m.cfg.Platforms[:idx], m.cfg.Platforms[idx+1:]...)
					saveConfig(m.cfg)
				}
				m.state = stateManageMenu
				m.cursor = 0
			default:
				m.state = stateManageMenu
				m.cursor = 0
			}
		}
		return m, nil
	}
	return m, nil
}

// Text input initialization for adding
func (m model) initTextInputs() model {
	ti := func(placeholder string) textinput.Model {
		t := textinput.New()
		t.Placeholder = placeholder
		t.CharLimit = 128
		t.Width = 30
		return t
	}
	m.textInputs = []textinput.Model{
		ti("Platform name"),
		ti("Client ID"),
		ti("Client Secret"),
		ti("OAuth URL"),
		ti("Base URL"),
		ti("Audience"),
	}
	m.focusIndex = 0
	m.textInputs[0].Focus()
	return m
}

// Text input initialization for editing
func (m model) initEditInputs(p Platform) model {
	ti := func(placeholder, value string) textinput.Model {
		t := textinput.New()
		t.Placeholder = placeholder
		t.CharLimit = 128
		t.Width = 30
		t.SetValue(value)
		return t
	}
	m.textInputs = []textinput.Model{
		ti("Platform name", p.Name),
		ti("Client ID", p.ClientID),
		ti("Client Secret", p.ClientSecret),
		ti("OAuth URL", p.OAuthURL),
		ti("Base URL", p.BaseURL),
		ti("Audience", p.Audience),
	}
	m.focusIndex = 0
	m.textInputs[0].Focus()
	return m
}

// === TOKEN CALL & MESSAGES ===
type tokenMsg string
type errMsg string

func getAccessToken(p Platform) (string, error) {
	data := map[string]string{
		"grant_type":    "client_credentials",
		"audience":      p.Audience,
		"client_id":     p.ClientID,
		"client_secret": p.ClientSecret,
	}
	body, _ := json.Marshal(data)
	req, err := http.NewRequest("POST", p.OAuthURL, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		responseData, _ := ioutil.ReadAll(resp.Body)
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(responseData))
	}
	var result struct {
		AccessToken string `json:"access_token"`
	}
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&result); err != nil {
		return "", err
	}
	return result.AccessToken, nil
}

// --- Fetch clusters with token ---
func fetchClusters(baseURL, token string) ([]Cluster, error) {
	url := baseURL + "/clusters"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		responseData, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(responseData))
	}
	var clusters []Cluster
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&clusters); err != nil {
		return nil, err
	}
	return clusters, nil
}

func (m model) View() string {
	s := ""
	switch m.state {
	case stateMainMenu:
		s += "Camunda CLI - Main Menu\n\n"
		for i, p := range m.cfg.Platforms {
			cursor := "  "
			if m.cursor == i {
				cursor = "➜ "
			}
			s += fmt.Sprintf("%s%s\n", cursor, p.Name)
		}
		add := len(m.cfg.Platforms)

		menu := []string{"Add new platform", "Manage platforms", "Quit"}
		for j, label := range menu {
			cursor := "  "
			if m.cursor == add+j {
				cursor = "➜ "
			}
			s += fmt.Sprintf("%s%s\n", cursor, label)
		}
		s += "\nPress q to quit."
	case stateAddPlatform:
		title := "Add New Platform"
		if m.state == stateAddPlatform && len(m.textInputs) > 0 && m.textInputs[0].Value() != "" {
			title = "Edit Platform"
		}
		s += title + "\n\n"
		fields := []string{
			"Platform name",
			"Client ID",
			"Client Secret",
			"OAuth URL",
			"Base URL",
			"Audience",
		}
		for i, ti := range m.textInputs {
			focus := " "
			if i == m.focusIndex {
				focus = "➜"
			}
			s += fmt.Sprintf("%s %s: %s\n", focus, fields[i], ti.View())
		}
		s += "\n(tab or enter to move, enter on last field to save, esc to cancel)"
	case stateShowToken:
		if m.errMsg != "" {
			s += "\n[ERROR] " + m.errMsg + "\n"
		} else {
			s += "\nAccess token:\n\n" + m.token + "\n"
		}
		s += "\nPress any key to list clusters."
	case stateListClusters:
		s += "Clusters on this platform:\n\n"
		if len(m.clusters) == 0 && m.errMsg == "" {
			s += "Loading...\n"
		} else if m.errMsg != "" {
			s += "[ERROR] " + m.errMsg + "\n"
		} else {
			for i, c := range m.clusters {
				cursor := "  "
				if m.cursor == i {
					cursor = "➜ "
				}
				s += fmt.Sprintf("%s%s (%s)\n", cursor, c.Name, c.Generation.Name)
			}
		}
		s += "\nPress q or esc to return to main menu."
	case stateManageMenu:
		s += "Manage Platforms\n\n"
		for i, p := range m.cfg.Platforms {
			cursor := "  "
			if m.cursor == i {
				cursor = "➜ "
			}
			s += fmt.Sprintf("%s%s\n", cursor, p.Name)
		}
		back := len(m.cfg.Platforms)
		cursor := "  "
		if m.cursor == back {
			cursor = "➜ "
		}
		s += fmt.Sprintf("%sBack\n", cursor)
		s += "\nEnter to edit/delete, esc to return."
	case stateEditPlatform:
		s += fmt.Sprintf("Platform: %s\n\n", m.cfg.Platforms[m.platformIndex].Name)
		options := []string{"Edit platform", "Delete platform"}
		for i, o := range options {
			cursor := "  "
			if m.cursor == i {
				cursor = "➜ "
			}
			s += fmt.Sprintf("%s%s\n", cursor, o)
		}
		s += "\nEsc to return."
	case stateConfirmDelete:
		s += fmt.Sprintf("\nDelete platform '%s'? (y/N)\n", m.cfg.Platforms[m.platformIndex].Name)
	case stateQuit:
		s += "Bye!\n"
	default:
		s += "Unknown state.\n"
	}
	return s
}

func main() {
	p := tea.NewProgram(initialModel())
	if err := p.Start(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}
