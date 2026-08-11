package api

import (
	"encoding/base64"
	"log"
	"strings"
	"time"

	"github.com/faroos/faroos/internal/proto"
	"golang.org/x/mod/semver"
)

func shouldBootstrapAgent(panelVersion, agentVersion string) bool {
	if panelVersion == "" || agentVersion == "" || panelVersion == agentVersion {
		return false
	}
	const devPrefix = "dev-"
	panelDev := strings.TrimPrefix(panelVersion, devPrefix)
	agentDev := strings.TrimPrefix(agentVersion, devPrefix)
	if panelDev != panelVersion && agentDev != agentVersion && len(panelDev) == 14 && len(agentDev) == 14 {
		return panelDev > agentDev
	}
	panelSemver := panelVersion
	if !strings.HasPrefix(panelSemver, "v") {
		panelSemver = "v" + panelSemver
	}
	agentSemver := agentVersion
	if !strings.HasPrefix(agentSemver, "v") {
		agentSemver = "v" + agentSemver
	}
	if semver.IsValid(panelSemver) && semver.IsValid(agentSemver) {
		return semver.Compare(panelSemver, agentSemver) > 0
	}
	// Unknown legacy version formats are treated as old so installations made
	// before versioned agents can still migrate to the current feed.
	return true
}

const remoteAgentBootstrapScript = `set -eu
. /etc/faroos/agent.env
case "$FAROOS_SERVER" in
  wss://*) panel_url="https://${FAROOS_SERVER#wss://}" ;;
  ws://*) panel_url="http://${FAROOS_SERVER#ws://}" ;;
  *) echo "unsupported FaroOS server URL" >&2; exit 1 ;;
esac
panel_url="${panel_url%/api/agent/connect}"
update_url="${panel_url%/}/install/update"

tmp_dir="$(mktemp -d /var/lib/faroos/update/bootstrap.XXXXXX)"
trap 'rm -rf "$tmp_dir"' EXIT
curl -fL --retry 3 --connect-timeout 15 -o "$tmp_dir/faroos-update" "${panel_url%/}/install/updater"
chmod 0755 "$tmp_dir/faroos-update"
install -d -o root -g root -m 0755 /etc/faroos /usr/local/libexec /etc/systemd/system/faroos-agent-update.timer.d
install -o root -g root -m 0755 "$tmp_dir/faroos-update" /usr/local/libexec/faroos-agent-update
printf 'FAROOS_UPDATE_CHANNEL="panel"\nFAROOS_UPDATE_URL="%s"\n' "$update_url" >"$tmp_dir/agent-update.env"
install -o root -g root -m 0644 "$tmp_dir/agent-update.env" /etc/faroos/agent-update.env
printf '%s\n' '[Timer]' 'OnBootSec=' 'OnBootSec=1min' 'OnUnitActiveSec=' 'OnUnitActiveSec=1min' 'RandomizedDelaySec=' 'RandomizedDelaySec=10s' >"$tmp_dir/panel.conf"
install -o root -g root -m 0644 "$tmp_dir/panel.conf" /etc/systemd/system/faroos-agent-update.timer.d/panel.conf
systemctl daemon-reload
systemctl enable faroos-agent-update.timer
systemctl restart faroos-agent-update.timer
systemctl start --no-block faroos-agent-update.service
`

// bootstrapRemoteAgentUpdate migrates agents installed before the panel feed
// existed. It uses the already-authenticated root terminal channel once; the
// installed timer handles every subsequent update without remote shell work.
func (s *Server) bootstrapRemoteAgentUpdate(nodeID, nodeName, oldVersion string, ac *agentConn) {
	sessionID := "update-" + newCommandID()
	stream := ac.openStream(sessionID)
	defer ac.closeStream(sessionID)

	if err := ac.writeEnvelope(proto.Envelope{
		Type:         proto.TypeTerminalOpen,
		TerminalOpen: &proto.TerminalOpen{SessionID: sessionID, Cols: 80, Rows: 24},
	}); err != nil {
		log.Printf("agent updater bootstrap for %s (%s): open terminal: %v", nodeName, nodeID, err)
		return
	}

	command := "/bin/sh -s <<'FAROOS_REMOTE_UPDATE'\n" + remoteAgentBootstrapScript + "\nFAROOS_REMOTE_UPDATE\nexit\n"
	if err := ac.writeEnvelope(proto.Envelope{
		Type: proto.TypeTerminalInput,
		TerminalData: &proto.TerminalData{
			SessionID: sessionID,
			DataB64:   base64.StdEncoding.EncodeToString([]byte(command)),
		},
	}); err != nil {
		log.Printf("agent updater bootstrap for %s (%s): send command: %v", nodeName, nodeID, err)
		return
	}

	timer := time.NewTimer(90 * time.Second)
	defer timer.Stop()
	for {
		select {
		case env, ok := <-stream:
			if !ok || env.Type == proto.TypeTerminalClose {
				log.Printf("agent updater bootstrap requested for %s (%s): %s -> %s", nodeName, nodeID, oldVersion, s.version)
				return
			}
		case <-timer.C:
			log.Printf("agent updater bootstrap for %s (%s): timed out", nodeName, nodeID)
			return
		}
	}
}
