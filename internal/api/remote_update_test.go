package api

import (
	"os/exec"
	"strings"
	"testing"
)

func TestRemoteAgentBootstrapScriptSyntax(t *testing.T) {
	cmd := exec.Command("sh", "-n")
	cmd.Stdin = strings.NewReader(remoteAgentBootstrapScript)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bootstrap script is invalid: %v\n%s", err, output)
	}
	for _, required := range []string{
		`FAROOS_UPDATE_CHANNEL="panel"`,
		`/install/update`,
		`systemctl start --no-block faroos-agent-update.service`,
	} {
		if !strings.Contains(remoteAgentBootstrapScript, required) {
			t.Fatalf("bootstrap script is missing %q", required)
		}
	}
}

func TestShouldBootstrapAgentNeverDowngradesKnownVersions(t *testing.T) {
	for _, test := range []struct {
		panel string
		agent string
		want  bool
	}{
		{panel: "dev-20260810215824", agent: "dev-20260810212716", want: true},
		{panel: "dev-20260810212716", agent: "dev-20260810215824", want: false},
		{panel: "v1.4.0", agent: "v1.3.9", want: true},
		{panel: "v1.3.9", agent: "v1.4.0", want: false},
		{panel: "v1.4.0", agent: "v1.4.0", want: false},
		{panel: "dev-20260810215824", agent: "0.0.1-dev", want: true},
	} {
		if got := shouldBootstrapAgent(test.panel, test.agent); got != test.want {
			t.Errorf("shouldBootstrapAgent(%q, %q) = %v, want %v", test.panel, test.agent, got, test.want)
		}
	}
}
