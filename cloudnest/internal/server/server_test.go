package server

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestGenerateInstallScriptSetsSystemdHome(t *testing.T) {
	script := generateInstallScript("https://example.com")

	requiredSnippets := []string{
		`AGENT_HOME="$(getent passwd root | cut -d: -f6 2>/dev/null || true)"`,
		`[ -n "$AGENT_HOME" ] || AGENT_HOME="/root"`,
		`AGENT_ENV_FILE="${AGENT_ETC_DIR}/agent.env"`,
		`SIGNING_SECRET_PATH="${AGENT_ETC_DIR}/signing_secret"`,
		`HOME="$AGENT_HOME" "${INSTALL_DIR}/cloudnest-agent" register \`,
		`--token-file "$REG_TOKEN_TMP_FILE" \`,
		`WorkingDirectory=${AGENT_HOME}`,
		`EnvironmentFile=${AGENT_ENV_FILE}`,
	}

	for _, snippet := range requiredSnippets {
		if !strings.Contains(script, snippet) {
			t.Fatalf("generated install script missing %q", snippet)
		}
	}
}

func TestGenerateInstallScriptHandlesBinaryUpgrade(t *testing.T) {
	script := generateInstallScript("https://example.com")

	requiredSnippets := []string{
		`TMP_BINARY="${INSTALL_DIR}/cloudnest-agent.tmp"`,
		`if systemctl list-unit-files "${SERVICE_NAME}.service" >/dev/null 2>&1; then`,
		`systemctl stop "$SERVICE_NAME" || true`,
		`curl -sSLf -o "${TMP_BINARY}" "${MASTER_URL}/download/agent/${OS}/${ARCH}"`,
		`mv "${TMP_BINARY}" "${INSTALL_DIR}/cloudnest-agent"`,
		`--token|--secret)`,
		`die "$1 is no longer supported for security reasons. Use --token-file/--secret-file or interactive input."`,
		`systemctl restart "$SERVICE_NAME"`,
	}

	for _, snippet := range requiredSnippets {
		if !strings.Contains(script, snippet) {
			t.Fatalf("generated install script missing %q", snippet)
		}
	}
}

func TestResolvePublicBaseURLUsesEnvOverride(t *testing.T) {
	req := httptest.NewRequest("GET", "http://ignored.example/install.sh", nil)
	req.Host = "ignored.example"
	req.Header.Set("X-Forwarded-Proto", "https")

	got := resolvePublicBaseURL(req, func(key string) string {
		if key == publicBaseURLEnvKey {
			return "https://ops.example.com/"
		}
		return ""
	})

	if got != "https://ops.example.com" {
		t.Fatalf("expected env override, got %q", got)
	}
}

func TestServeAgentBinaryUsesExecutableDir(t *testing.T) {
	t.Setenv("GIN_MODE", "release")
	gin.SetMode(gin.ReleaseMode)

	tmpDir := t.TempDir()
	binaryDir := filepath.Join(tmpDir, "data", "binaries")
	if err := os.MkdirAll(binaryDir, 0o755); err != nil {
		t.Fatalf("mkdir binary dir: %v", err)
	}
	expectedContent := []byte("agent-binary")
	if err := os.WriteFile(filepath.Join(binaryDir, "cloudnest-agent-linux-amd64"), expectedContent, 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}

	originalExecutablePathFunc := executablePathFunc
	executablePathFunc = func() (string, error) {
		return filepath.Join(tmpDir, "cloudnest"), nil
	}
	defer func() {
		executablePathFunc = originalExecutablePathFunc
	}()

	router := gin.New()
	router.GET("/download/agent/:os/:arch", serveAgentBinary)

	req := httptest.NewRequest("GET", "/download/agent/linux/amd64", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if body := recorder.Body.String(); body != string(expectedContent) {
		t.Fatalf("unexpected body %q", body)
	}
}

func TestHealthzReturnsOK(t *testing.T) {
	t.Setenv("GIN_MODE", "release")
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.GET("/healthz", healthz)

	req := httptest.NewRequest("GET", "/healthz", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != 200 {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if body := recorder.Body.String(); !strings.Contains(body, `"status":"ok"`) {
		t.Fatalf("unexpected body %q", body)
	}
}
