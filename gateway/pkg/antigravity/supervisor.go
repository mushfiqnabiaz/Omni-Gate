package antigravity

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/antigravity/gateway/pkg/models"
)

const (
	KeychainService = "gemini"
	KeychainAccount = "antigravity"
	JetskiFile      = "jetski-standalone-oauth-token"
)

type LanguageServerInfo struct {
	PID                    int      `json:"pid"`
	CSRF                   string   `json:"csrf"`
	Ports                  []int    `json:"ports"`
	Email                  string   `json:"email"`
	Name                   string   `json:"name"`
	Tier                   string   `json:"tier"`
	AppType                string   `json:"app_type"`
	Connected              bool     `json:"connected"`
	ModelsCount            int      `json:"models_count"`
	QuotaRemainingFraction float64  `json:"quota_remaining_fraction"`
}

type Supervisor struct {
	homeDir    string
	httpClient *http.Client
}

func NewSupervisor() *Supervisor {
	home, _ := os.UserHomeDir()
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	return &Supervisor{
		homeDir: home,
		httpClient: &http.Client{
			Transport: tr,
			Timeout:   2500 * time.Millisecond,
		},
	}
}

// BuildJetskiDocument creates standard token payload for Antigravity IDE
func BuildJetskiDocument(creds *models.GoogleCredentials) map[string]any {
	var expTime time.Time
	if creds.ExpiryTimestamp <= 0 {
		expTime = time.Now().Add(1 * time.Hour).UTC()
	} else if creds.ExpiryTimestamp > 1e11 {
		expTime = time.UnixMilli(creds.ExpiryTimestamp).UTC()
	} else {
		expTime = time.Unix(creds.ExpiryTimestamp, 0).UTC()
	}
	rfc3339 := expTime.Format("2006-01-02T15:04:05.000000Z")

	return map[string]any{
		"token": map[string]any{
			"access_token":  creds.AccessToken,
			"token_type":    "Bearer",
			"refresh_token": creds.RefreshToken,
			"expiry":        rfc3339,
		},
		"auth_method": "consumer",
	}
}

// WriteKeychainToken safely writes OAuth tokens to macOS Keychain
func (s *Supervisor) WriteKeychainToken(creds *models.GoogleCredentials) error {
	if runtime.GOOS != "darwin" {
		return nil
	}

	doc := BuildJetskiDocument(creds)
	payloadJson, err := json.Marshal(doc)
	if err != nil {
		return err
	}

	b64 := base64.StdEncoding.EncodeToString(payloadJson)
	keyringVal := fmt.Sprintf("go-keyring-base64:%s", b64)

	// Clean existing password
	_ = exec.Command("security", "delete-generic-password", "-s", KeychainService, "-a", KeychainAccount).Run()

	// Write new password with -A (allows all applications without UI prompt)
	cmd := exec.Command("security", "add-generic-password",
		"-s", KeychainService,
		"-a", KeychainAccount,
		"-l", "Antigravity",
		"-w", keyringVal,
		"-A",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("keychain write failed: %v, output: %s", err, string(output))
	}

	log.Println("[Antigravity] 🔑 Successfully updated macOS Keychain credentials")
	return nil
}

// WriteJetskiToken writes ~/.gemini/jetski-standalone-oauth-token
func (s *Supervisor) WriteJetskiToken(creds *models.GoogleCredentials) error {
	geminiDir := filepath.Join(s.homeDir, ".gemini")
	if err := os.MkdirAll(geminiDir, 0700); err != nil {
		return err
	}

	target := filepath.Join(geminiDir, JetskiFile)
	tmp := fmt.Sprintf("%s.%d.tmp", target, os.Getpid())

	doc := BuildJetskiDocument(creds)
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}

	if err := os.Rename(tmp, target); err != nil {
		return err
	}

	_ = os.Chmod(target, 0600)
	log.Printf("[Antigravity] 📄 Updated %s\n", target)

	// Also sync google_accounts.json
	accFile := filepath.Join(geminiDir, "google_accounts.json")
	var accDoc map[string]any
	if d, err := os.ReadFile(accFile); err == nil {
		_ = json.Unmarshal(d, &accDoc)
	}
	if accDoc == nil {
		accDoc = make(map[string]any)
	}
	accDoc["active"] = creds.ProjectID // fallback
	if creds.AccessToken != "" {
		accBytes, _ := json.MarshalIndent(accDoc, "", "  ")
		_ = os.WriteFile(accFile, accBytes, 0600)
	}

	return nil
}

func (s *Supervisor) parseLanguageServerLine(line string, appType string) *LanguageServerInfo {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return nil
	}
	pid, err := strconv.Atoi(fields[0])
	if err != nil {
		return nil
	}

	reCSRF := regexp.MustCompile(`--csrf_token\s+(\S+)`)
	matchCSRF := reCSRF.FindStringSubmatch(line)
	if len(matchCSRF) < 2 {
		return nil
	}
	csrf := matchCSRF[1]

	lsofCmd := "lsof"
	if _, err := os.Stat("/usr/sbin/lsof"); err == nil {
		lsofCmd = "/usr/sbin/lsof"
	}
	lsofOut, err := exec.Command(lsofCmd, "-a", "-nP", "-p", strconv.Itoa(pid), "-iTCP", "-sTCP:LISTEN").Output()
	var ports []int
	if err == nil {
		rePort := regexp.MustCompile(`127\.0\.0\.1:(\d+)\s+\(LISTEN\)`)
		matches := rePort.FindAllStringSubmatch(string(lsofOut), -1)
		for _, m := range matches {
			if len(m) > 1 {
				p, _ := strconv.Atoi(m[1])
				if p > 0 {
					ports = append(ports, p)
				}
			}
		}
	}

	info := &LanguageServerInfo{
		PID:       pid,
		CSRF:      csrf,
		Ports:     ports,
		AppType:   appType,
		Connected: true,
	}

	s.populateUserStatus(info)
	return info
}

// DiscoverAll discovers both active Antigravity IDE and Antigravity Desktop language servers
func (s *Supervisor) DiscoverAll() (*LanguageServerInfo, *LanguageServerInfo, error) {
	cmd := exec.Command("ps", "-ax", "-o", "pid=,command=")
	out, err := cmd.Output()
	if err != nil {
		return nil, nil, fmt.Errorf("ps command failed: %w", err)
	}

	lines := strings.Split(string(out), "\n")
	var ideLine, desktopLine string

	for _, l := range lines {
		if ideLine == "" && strings.Contains(l, "language_server_macos_arm") && strings.Contains(l, "--csrf_token") {
			ideLine = l
		}
		if desktopLine == "" && (strings.Contains(l, "/Applications/Antigravity.app") || strings.Contains(l, "--subclient_type hub")) && strings.Contains(l, "--csrf_token") {
			desktopLine = l
		}
	}

	var ideInfo, desktopInfo *LanguageServerInfo
	if ideLine != "" {
		ideInfo = s.parseLanguageServerLine(ideLine, "Antigravity IDE")
	}
	if desktopLine != "" {
		desktopInfo = s.parseLanguageServerLine(desktopLine, "Antigravity Desktop")
	}

	return ideInfo, desktopInfo, nil
}

// DiscoverLanguageServer returns primary detected language server (IDE prioritized, then Desktop)
func (s *Supervisor) DiscoverLanguageServer() (*LanguageServerInfo, error) {
	ide, desktop, err := s.DiscoverAll()
	if ide != nil && ide.Connected {
		return ide, err
	}
	if desktop != nil && desktop.Connected {
		return desktop, err
	}
	return &LanguageServerInfo{Connected: false, AppType: "None Detected"}, err
}

// DiscoverDesktopLanguageServer returns Antigravity Desktop language server info specifically
func (s *Supervisor) DiscoverDesktopLanguageServer() (*LanguageServerInfo, error) {
	_, desktop, err := s.DiscoverAll()
	if desktop != nil && desktop.Connected {
		return desktop, err
	}
	return &LanguageServerInfo{Connected: false, AppType: "Antigravity Desktop"}, err
}

// populateUserStatus calls LanguageServerService/GetUserStatus over Connect protocol
func (s *Supervisor) populateUserStatus(info *LanguageServerInfo) {
	for _, port := range info.Ports {
		for _, scheme := range []string{"http", "https"} {
			url := fmt.Sprintf("%s://127.0.0.1:%d/exa.language_server_pb.LanguageServerService/GetUserStatus", scheme, port)
			req, err := http.NewRequestWithContext(context.Background(), "POST", url, bytes.NewBuffer([]byte("{}")))
			if err != nil {
				continue
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("connect-protocol-version", "1")
			req.Header.Set("x-codeium-csrf-token", info.CSRF)

			resp, err := s.httpClient.Do(req)
			if err != nil {
				continue
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				var statusResp struct {
					UserStatus struct {
						Name     string `json:"name"`
						Email    string `json:"email"`
						Tier     string `json:"tier"`
						UserTier struct {
							Name string `json:"name"`
						} `json:"userTier"`
						CascadeModelConfig struct {
							ClientModels []struct {
								ModelID   string `json:"modelId"`
								QuotaInfo struct {
									RemainingFraction float64 `json:"remainingFraction"`
								} `json:"quotaInfo"`
							} `json:"clientModels"`
						} `json:"cascadeModelConfig"`
					} `json:"userStatus"`
				}
				if err := json.Unmarshal(body, &statusResp); err == nil && statusResp.UserStatus.Email != "" {
					info.Email = statusResp.UserStatus.Email
					info.Name = statusResp.UserStatus.Name
					if statusResp.UserStatus.UserTier.Name != "" {
						info.Tier = statusResp.UserStatus.UserTier.Name
					} else {
						info.Tier = statusResp.UserStatus.Tier
					}
					modelsList := statusResp.UserStatus.CascadeModelConfig.ClientModels
					info.ModelsCount = len(modelsList)
					if len(modelsList) > 0 {
						for _, m := range modelsList {
							if m.QuotaInfo.RemainingFraction > 0 {
								info.QuotaRemainingFraction = m.QuotaInfo.RemainingFraction
								break
							}
						}
					}
					return
				}
			}
		}
	}
}

// PreserveConversationLayout preserves active tab and conversation state
func (s *Supervisor) PreserveConversationLayout() error {
	appStoragePath := filepath.Join(s.homeDir, "Library", "Application Support", "Antigravity", "app_storage.json")
	if _, err := os.Stat(appStoragePath); os.IsNotExist(err) {
		return nil
	}

	content, err := os.ReadFile(appStoragePath)
	if err != nil {
		return err
	}

	var storage map[string]any
	if err := json.Unmarshal(content, &storage); err != nil {
		return err
	}

	// Backup current layout before language server reload
	backupPath := fmt.Sprintf("%s.backup-%d", appStoragePath, time.Now().Unix())
	_ = os.WriteFile(backupPath, content, 0644)
	log.Printf("[Antigravity] 💾 Preserved Antigravity app layout backup at %s\n", backupPath)
	return nil
}

func encodeVarint(v uint64) []byte {
	var buf []byte
	for {
		b := byte(v & 0x7f)
		v >>= 7
		if v != 0 {
			buf = append(buf, b|0x80)
		} else {
			buf = append(buf, b)
			break
		}
	}
	return buf
}

func encodeLengthDelimited(fieldNum int, data []byte) []byte {
	tag := byte((fieldNum << 3) | 2)
	buf := []byte{tag}
	buf = append(buf, encodeVarint(uint64(len(data)))...)
	buf = append(buf, data...)
	return buf
}

func buildOAuthTokenProto(creds *models.GoogleCredentials) string {
	var buf []byte
	if creds.AccessToken != "" {
		buf = append(buf, encodeLengthDelimited(1, []byte(creds.AccessToken))...)
	}
	buf = append(buf, encodeLengthDelimited(2, []byte("Bearer"))...)
	if creds.RefreshToken != "" {
		buf = append(buf, encodeLengthDelimited(3, []byte(creds.RefreshToken))...)
	}
	var expSec int64 = creds.ExpiryTimestamp
	if expSec <= 0 {
		expSec = time.Now().Add(1 * time.Hour).Unix()
	} else if expSec > 1e11 {
		expSec = expSec / 1000
	}
	expiryMsg := []byte{0x08}
	expiryMsg = append(expiryMsg, encodeVarint(uint64(expSec))...)
	buf = append(buf, encodeLengthDelimited(4, expiryMsg)...)

	return base64.StdEncoding.EncodeToString(buf)
}

func buildTopicDoc(tokenB64 string) string {
	authStateJSON := `{"state":"signedIn","context":{"project":"","showProjectError":false,"errorMessage":"","ineligibleMessage":"","verificationUrl":"","isGcpTos":false,"browserOpenFailed":false,"appealUrl":"","appealLinkText":""}}`

	buildEntry := func(key string, val string) []byte {
		rowBytes := encodeLengthDelimited(1, []byte(val))
		var entry []byte
		entry = append(entry, encodeLengthDelimited(1, []byte(key))...)
		entry = append(entry, encodeLengthDelimited(2, rowBytes)...)
		return encodeLengthDelimited(1, entry)
	}

	var topic []byte
	topic = append(topic, buildEntry("authStateWithContextSentinelKey", authStateJSON)...)
	topic = append(topic, buildEntry("oauthTokenInfoSentinelKey", tokenB64)...)
	return base64.StdEncoding.EncodeToString(topic)
}

// WriteIdeStateDB writes credentials directly to Antigravity IDE SQLite state database
func (s *Supervisor) WriteIdeStateDB(creds *models.GoogleCredentials) error {
	dbPath := filepath.Join(s.homeDir, "Library", "Application Support", "Antigravity IDE", "User", "globalStorage", "state.vscdb")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil
	}

	tokenB64 := buildOAuthTokenProto(creds)
	topicB64 := buildTopicDoc(tokenB64)

	query := fmt.Sprintf("INSERT OR REPLACE INTO ItemTable (key, value) VALUES ('antigravityUnifiedStateSync.oauthToken', '%s');", topicB64)
	cmd := exec.Command("sqlite3", dbPath, query)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to update state.vscdb: %w (output: %s)", err, string(out))
	}

	log.Println("[Antigravity] 💾 Successfully updated Antigravity IDE state.vscdb")
	return nil
}

// PushIdeUnifiedStateUpdate pushes live OAuth token updates to running Antigravity IDE extension server
func (s *Supervisor) PushIdeUnifiedStateUpdate(creds *models.GoogleCredentials) error {
	cmd := exec.Command("ps", "-ax", "-o", "pid=,command=")
	out, err := cmd.Output()
	if err != nil {
		return err
	}

	reExtPort := regexp.MustCompile(`--extension_server_port\s+(\d+)`)
	reExtCSRF := regexp.MustCompile(`--extension_server_csrf_token\s+(\S+)`)

	var extPort int
	var extCSRF string

	for _, l := range strings.Split(string(out), "\n") {
		if strings.Contains(l, "language_server_macos_arm") && strings.Contains(l, "--extension_server_port") {
			mPort := reExtPort.FindStringSubmatch(l)
			mCSRF := reExtCSRF.FindStringSubmatch(l)
			if len(mPort) > 1 && len(mCSRF) > 1 {
				extPort, _ = strconv.Atoi(mPort[1])
				extCSRF = mCSRF[1]
				break
			}
		}
	}

	if extPort == 0 || extCSRF == "" {
		return nil
	}

	tokenB64 := buildOAuthTokenProto(creds)

	pushURL := fmt.Sprintf("http://127.0.0.1:%d/exa.extension_server_pb.ExtensionServerService/PushUnifiedStateSyncUpdate", extPort)
	payload := map[string]any{
		"update": map[string]any{
			"topicName": "uss-oauth",
			"appliedUpdate": map[string]any{
				"key": "oauthTokenInfoSentinelKey",
				"newRow": map[string]any{
					"value": tokenB64,
				},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(context.Background(), "POST", pushURL, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("connect-protocol-version", "1")
	req.Header.Set("x-codeium-csrf-token", extCSRF)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("push update failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		log.Printf("[Antigravity] 📡 Pushed live OAuth update to extension server on port %d\n", extPort)
	}
	return nil
}

// RestartLanguageServer restarts the language_server cleanly or gracefully relaunches the IDE
func (s *Supervisor) RestartLanguageServer(force bool) error {
	info, err := s.DiscoverLanguageServer()
	if err != nil || !info.Connected {
		return fmt.Errorf("language server not connected: %v", err)
	}

	_ = s.PreserveConversationLayout()

	// IMPORTANT: For Antigravity IDE (VS Code based), never send SIGKILL or exit RPCs to the language server
	// subprocess, as that causes extension crash popups. If force is requested, gracefully relaunch the IDE app.
	if info.AppType == "Antigravity IDE" {
		if force {
			log.Println("[Antigravity] 🔄 Gracefully relaunching Antigravity IDE app...")
			_ = exec.Command("osascript", "-e", "quit application id \"com.google.antigravity-ide\"").Run()
			time.Sleep(1200 * time.Millisecond)
			_ = exec.Command("open", "-b", "com.google.antigravity-ide").Run()
			return nil
		}
		log.Println("[Antigravity] ℹ️ Antigravity IDE detected: Credentials safely synchronized to Keychain, ~/.gemini, and state.vscdb.")
		return nil
	}

	proc, err := os.FindProcess(info.PID)
	if err != nil {
		return err
	}

	log.Printf("[Antigravity] 🔄 Restarting %s (PID %d)...", info.AppType, info.PID)
	if err := proc.Kill(); err != nil {
		return fmt.Errorf("failed to kill language_server: %w", err)
	}

	time.Sleep(1800 * time.Millisecond)
	newInfo, err := s.DiscoverLanguageServer()
	if err == nil && newInfo.Connected && newInfo.PID != info.PID {
		log.Printf("[Antigravity] ✅ %s re-spawned successfully with new PID %d\n", newInfo.AppType, newInfo.PID)
	}

	return nil
}

// WriteDesktopStorage updates ~/Library/Application Support/Antigravity/app_storage.json
func (s *Supervisor) WriteDesktopStorage(email string) error {
	storagePath := filepath.Join(s.homeDir, "Library", "Application Support", "Antigravity", "app_storage.json")
	if _, err := os.Stat(storagePath); os.IsNotExist(err) {
		return nil
	}

	content, err := os.ReadFile(storagePath)
	if err != nil {
		return err
	}

	var storage map[string]any
	if err := json.Unmarshal(content, &storage); err != nil {
		return err
	}

	storage["jetski.onboarding.lastLoginUsername"] = email
	updated, err := json.MarshalIndent(storage, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(storagePath, updated, 0644); err != nil {
		return err
	}
	log.Printf("[Antigravity] 💾 Updated Antigravity Desktop app_storage.json with email: %s\n", email)
	return nil
}

// RestartDesktopLanguageServer restarts Antigravity Desktop's language server cleanly
func (s *Supervisor) RestartDesktopLanguageServer() error {
	info, err := s.DiscoverDesktopLanguageServer()
	if err != nil || !info.Connected {
		log.Println("[Antigravity] ℹ️ Antigravity Desktop not currently running; skipping desktop restart")
		return nil
	}

	_ = s.PreserveConversationLayout()

	// 1. Attempt graceful Connect RPC Restart over HTTPS/HTTP
	for _, port := range info.Ports {
		for _, scheme := range []string{"https", "http"} {
			url := fmt.Sprintf("%s://127.0.0.1:%d/exa.language_server_pb.LanguageServerService/Restart", scheme, port)
			req, err := http.NewRequestWithContext(context.Background(), "POST", url, bytes.NewBuffer([]byte("{}")))
			if err != nil {
				continue
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("connect-protocol-version", "1")
			req.Header.Set("x-codeium-csrf-token", info.CSRF)

			resp, err := s.httpClient.Do(req)
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					log.Printf("[Antigravity] 🔄 Sent RPC Restart to Antigravity Desktop (PID %d)\n", info.PID)
					time.Sleep(2500 * time.Millisecond)
					return nil
				}
			}
		}
	}

	// 2. Fallback to process termination (SIGTERM)
	proc, err := os.FindProcess(info.PID)
	if err != nil {
		return err
	}
	log.Printf("[Antigravity] 🔄 Signaling Antigravity Desktop (PID %d) to re-spawn...", info.PID)
	_ = proc.Signal(syscall.SIGTERM)
	time.Sleep(2000 * time.Millisecond)
	return nil
}

// ApplyAccountSwitch atomically writes Keychain, ~/.gemini tokens, state.vscdb, USS, and restarts Desktop and/or IDE
func (s *Supervisor) ApplyAccountSwitch(creds *models.GoogleCredentials, accountEmail string, target string, forceRestart bool) error {
	if err := s.WriteKeychainToken(creds); err != nil {
		return err
	}

	if err := s.WriteJetskiToken(creds); err != nil {
		return err
	}

	target = strings.ToLower(strings.TrimSpace(target))

	// 1. Target IDE (default or if target includes ide/both/all)
	if target == "" || target == "ide" || target == "both" || target == "all" {
		if err := s.WriteIdeStateDB(creds); err != nil {
			log.Printf("[Antigravity] ⚠️ Could not write state.vscdb: %v\n", err)
		}

		if err := s.PushIdeUnifiedStateUpdate(creds); err != nil {
			log.Printf("[Antigravity] ⚠️ Could not push live USS update: %v\n", err)
		}

		if forceRestart {
			_ = s.RestartLanguageServer(true)
		}
	}

	// 2. Target Desktop (default or if target includes desktop/both/all)
	if target == "" || target == "desktop" || target == "both" || target == "all" {
		if accountEmail != "" {
			if err := s.WriteDesktopStorage(accountEmail); err != nil {
				log.Printf("[Antigravity] ⚠️ Could not write Desktop app_storage.json: %v\n", err)
			}
		}

		if err := s.RestartDesktopLanguageServer(); err != nil {
			log.Printf("[Antigravity] ⚠️ Could not restart Desktop language server: %v\n", err)
		}
	}

	return nil
}
