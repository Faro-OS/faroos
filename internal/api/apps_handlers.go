package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/faroos/faroos/internal/appcatalog"
	"github.com/faroos/faroos/internal/proto"
)

// deployTimeoutDuration is longer than the default command timeout since
// deploying an app may need to pull a large image first.
const deployTimeoutDuration = 6 * time.Minute

func (s *Server) handleListApps(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, appcatalog.Catalog)
}

type appIDParams struct {
	AppID string `json:"appId"`
}

func (s *Server) handleDeployApp(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("appId")
	if _, ok := appcatalog.Find(appID); !ok {
		http.Error(w, "unknown app", http.StatusNotFound)
		return
	}
	ac, ok := s.resolveConnectedNode(w, r.PathValue("id"))
	if !ok {
		return
	}
	params, _ := json.Marshal(appIDParams{AppID: appID})

	result, err := ac.sendWithTimeout(proto.Command{ID: newCommandID(), Action: "apps.deploy", Params: params}, deployTimeoutDuration)
	if err != nil {
		http.Error(w, err.Error(), http.StatusGatewayTimeout)
		return
	}
	if !result.OK {
		http.Error(w, result.Error, http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleRemoveApp(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("appId")
	ac, ok := s.resolveConnectedNode(w, r.PathValue("id"))
	if !ok {
		return
	}
	params, _ := json.Marshal(appIDParams{AppID: appID})

	result, err := ac.send(proto.Command{ID: newCommandID(), Action: "apps.remove", Params: params})
	if err != nil {
		http.Error(w, err.Error(), http.StatusGatewayTimeout)
		return
	}
	if !result.OK {
		http.Error(w, result.Error, http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}
