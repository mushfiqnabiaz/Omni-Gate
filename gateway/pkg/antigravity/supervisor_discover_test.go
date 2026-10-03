package antigravity

import (
	"strings"
	"testing"

	"github.com/antigravity/gateway/pkg/models"
)

const samplePS = `
30141 /Applications/Antigravity IDE.app/Contents/Resources/app/extensions/antigravity/bin/language_server_macos_arm --csrf_token parent --extension_server_port 65062 --extension_server_csrf_token parent-csrf --app_data_dir antigravity-ide --subclient_type ide
37458 /Applications/Antigravity IDE.app/Contents/Resources/app/extensions/antigravity/bin/language_server_macos_arm --enable_lsp --csrf_token workspace --extension_server_port 65099 --extension_server_csrf_token workspace-csrf --https_server_port 65100 --workspace_id file_Users_nabiaz_omnigate --subclient_type ide --app_data_dir antigravity-ide
39919 /Applications/Antigravity.app/Contents/Resources/bin/language_server --standalone --subclient_type hub --csrf_token hub --app_data_dir antigravity
`

func TestSplitLanguageServerLinesKeepsIDEAndDesktop(t *testing.T) {
	ide, desktop := splitLanguageServerLines(samplePS)
	if len(ide) != 2 {
		t.Fatalf("ide servers = %d, want 2", len(ide))
	}
	if len(desktop) != 1 {
		t.Fatalf("desktop servers = %d, want 1", len(desktop))
	}
	if !strings.Contains(ide[0], "--enable_lsp") {
		t.Fatalf("live IDE server should be chosen first, got %s", ide[0])
	}
	if classifyLanguageServerLine(desktop[0]) != "desktop" {
		t.Fatal("hub process was not classified as desktop")
	}
	if strings.Contains(ide[0], "--subclient_type hub") || strings.Contains(ide[1], "--subclient_type hub") {
		t.Fatal("desktop hub was classified as an IDE server")
	}
}

func TestExtensionTargetsIncludeEveryIDEServer(t *testing.T) {
	targets := extensionTargetsFromPS(samplePS)
	if len(targets) != 2 {
		t.Fatalf("extension targets = %d, want 2", len(targets))
	}
	if targets[0].port != 65099 || targets[1].port != 65062 {
		t.Fatalf("ports = %d,%d, want workspace 65099 then parent 65062", targets[0].port, targets[1].port)
	}
}

func TestSwitchTargetsBothUpdatesIDEAndDesktop(t *testing.T) {
	ide, desktop := switchTargets("both")
	if !ide || !desktop {
		t.Fatalf("both = ide %v desktop %v", ide, desktop)
	}
	ide, desktop = switchTargets("")
	if !ide || !desktop {
		t.Fatalf("default = ide %v desktop %v", ide, desktop)
	}
	ide, desktop = switchTargets("ide")
	if !ide || desktop {
		t.Fatalf("ide only = ide %v desktop %v", ide, desktop)
	}
}

func TestJetskiDocumentCarriesIDTokenForBothClients(t *testing.T) {
	doc := BuildJetskiDocument(&models.GoogleCredentials{
		AccessToken:     "access",
		RefreshToken:    "refresh",
		IdToken:         "id-token",
		ExpiryTimestamp: 1790413779,
	})
	if doc["id_token"] != "id-token" {
		t.Fatalf("id_token = %#v", doc["id_token"])
	}
	token := doc["token"].(map[string]any)
	if token["access_token"] != "access" || token["refresh_token"] != "refresh" {
		t.Fatalf("token document = %#v", token)
	}
}
