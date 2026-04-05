package server

import (
	"strings"
	"testing"
)

func TestGenerateInstallScriptSetsSystemdHome(t *testing.T) {
	script := generateInstallScript("https://example.com")

	requiredSnippets := []string{
		`AGENT_HOME="$(getent passwd root | cut -d: -f6 2>/dev/null || true)"`,
		`[ -n "$AGENT_HOME" ] || AGENT_HOME="/root"`,
		`HOME="$AGENT_HOME" "${INSTALL_DIR}/cloudnest-agent" register \`,
		`WorkingDirectory=${AGENT_HOME}`,
		`Environment=HOME=${AGENT_HOME}`,
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
		`systemctl restart "$SERVICE_NAME"`,
	}

	for _, snippet := range requiredSnippets {
		if !strings.Contains(script, snippet) {
			t.Fatalf("generated install script missing %q", snippet)
		}
	}
}
