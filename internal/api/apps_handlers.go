package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/faroos/faroos/internal/appcatalog"
	"github.com/faroos/faroos/internal/proto"
)

// deployTimeoutDuration is longer than the default command timeout since
// deploying an app may need to pull a large image first.
const deployTimeoutDuration = 6 * time.Minute

func (s *Server) handleListApps(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.catalog.List())
}

func (s *Server) handleAppCategories(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.catalog.Categories())
}

func (s *Server) handleRefreshApps(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Minute)
	defer cancel()
	if err := s.catalog.Refresh(ctx); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "count": len(s.catalog.List())})
}

type appIDParams struct {
	AppID string `json:"appId"`
}

// deployRequest lets the caller fully customize a deploy — the install
// dialog always sends one, pre-filled with the catalog's defaults and
// edited by the user (port, env values). Kept optional-shaped (all zero
// values valid) so nothing breaks if a client sends an empty body.
type deployRequest struct {
	Ports     []appcatalog.Port   `json:"ports"`
	Env       []appcatalog.EnvVar `json:"env"`
	Arguments *string             `json:"arguments"`
}

func (s *Server) handleDeployApp(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("appId")
	app, ok := s.catalog.Find(appID)
	if !ok {
		http.Error(w, "unknown app", http.StatusNotFound)
		return
	}
	ac, ok := s.resolveConnectedNode(w, r.PathValue("id"))
	if !ok {
		return
	}

	var req deployRequest
	if r.Body != nil {
		json.NewDecoder(r.Body).Decode(&req) // a missing/empty body just leaves req zeroed
	}

	ports := app.Ports
	if len(req.Ports) > 0 {
		ports = req.Ports
	}
	env := app.Env
	if len(req.Env) > 0 {
		env = req.Env
	}
	arguments := app.Arguments
	if req.Arguments != nil {
		arguments = *req.Arguments
	}
	if appcatalog.HasUnresolvedCredentialPlaceholder(arguments) {
		http.Error(w, "replace the credential placeholder in container arguments", http.StatusBadRequest)
		return
	}
	command, err := appcatalog.ParseArguments(arguments)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	for _, p := range ports {
		if p.Host < 1 || p.Host > 65535 {
			http.Error(w, "port must be between 1 and 65535", http.StatusBadRequest)
			return
		}
	}

	spec := appcatalog.DeploySpec{
		AppID:   app.ID,
		Image:   app.Image,
		Ports:   ports,
		Volumes: app.Volumes,
		Env:     env,
		Command: command,
	}
	params, _ := json.Marshal(spec)

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

type portParams struct {
	Port int `json:"port"`
}

func (s *Server) handleInspectPort(w http.ResponseWriter, r *http.Request) {
	port, err := strconv.Atoi(r.PathValue("port"))
	if err != nil || port < 1 || port > 65535 {
		http.Error(w, "invalid port", http.StatusBadRequest)
		return
	}
	ac, ok := s.resolveConnectedNode(w, r.PathValue("id"))
	if !ok {
		return
	}
	params, _ := json.Marshal(portParams{Port: port})

	result, err := ac.send(proto.Command{ID: newCommandID(), Action: "ports.inspect", Params: params})
	if err != nil {
		http.Error(w, err.Error(), http.StatusGatewayTimeout)
		return
	}
	if !result.OK {
		http.Error(w, result.Error, http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(result.Result)
}
