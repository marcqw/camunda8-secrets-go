package main

import (
    "bufio"
    "encoding/json"
    "fmt"
    "io/ioutil"
    "net/http"
    "os"
    "strings"
    "bytes"
)

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

type TokenResponse struct {
    AccessToken string `json:"access_token"`
}

const configFile = "camunda_cli_config.json"

func main() {
    config := loadOrInitConfig()

    idx := selectPlatform(config)
    if idx == -1 {
        // Add a new platform
        config.Platforms = append(config.Platforms, promptNewPlatform())
        saveConfig(config)
        idx = len(config.Platforms) - 1
    }

    platform := config.Platforms[idx]
    token, err := getAccessToken(platform)
    if err != nil {
        fmt.Println("Error retrieving access token:", err)
        return
    }
    fmt.Println("\nYour access token:")
    fmt.Println(token)
    // Use the token for further API calls if needed...
}

func loadOrInitConfig() Config {
    var config Config
    if _, err := os.Stat(configFile); err == nil {
        data, _ := ioutil.ReadFile(configFile)
        json.Unmarshal(data, &config)
    }
    if len(config.Platforms) == 0 {
        fmt.Println("No platforms configured yet.")
        config.Platforms = append(config.Platforms, promptNewPlatform())
        saveConfig(config)
    }
    return config
}

func promptNewPlatform() Platform {
    reader := bufio.NewReader(os.Stdin)
    fmt.Println("\nPlease enter the new Camunda platform information:")
    fmt.Print("Platform name: ")
    name, _ := reader.ReadString('\n')
    fmt.Print("CAMUNDA_CONSOLE_CLIENT_ID: ")
    clientID, _ := reader.ReadString('\n')
    fmt.Print("CAMUNDA_CONSOLE_CLIENT_SECRET: ")
    clientSecret, _ := reader.ReadString('\n')
    fmt.Print("CAMUNDA_OAUTH_URL: ")
    oauthURL, _ := reader.ReadString('\n')
    fmt.Print("CAMUNDA_CONSOLE_BASE_URL: ")
    baseURL, _ := reader.ReadString('\n')
    fmt.Print("CAMUNDA_CONSOLE_OAUTH_AUDIENCE: ")
    audience, _ := reader.ReadString('\n')

    return Platform{
        Name:         strings.TrimSpace(name),
        ClientID:     strings.TrimSpace(clientID),
        ClientSecret: strings.TrimSpace(clientSecret),
        OAuthURL:     strings.TrimSpace(oauthURL),
        BaseURL:      strings.TrimSpace(baseURL),
        Audience:     strings.TrimSpace(audience),
    }
}

func saveConfig(config Config) {
    data, _ := json.MarshalIndent(config, "", "  ")
    ioutil.WriteFile(configFile, data, 0600)
}

func selectPlatform(config Config) int {
    fmt.Println("\nAvailable platforms:")
    for i, p := range config.Platforms {
        fmt.Printf("  [%d] %s\n", i+1, p.Name)
    }
    fmt.Println("  [0] Add a new platform")
    fmt.Print("Select a platform: ")
    var choice int
    fmt.Scanf("%d\n", &choice)
    if choice == 0 {
        return -1
    }
    if choice > 0 && choice <= len(config.Platforms) {
        return choice - 1
    }
    fmt.Println("Invalid choice.")
    return selectPlatform(config)
}

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

    var tr TokenResponse
    decoder := json.NewDecoder(resp.Body)
    if err := decoder.Decode(&tr); err != nil {
        return "", err
    }
    return tr.AccessToken, nil
}