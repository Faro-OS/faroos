package api

import (
	"net/http"
	"time"

	"github.com/faroos/faroos/internal/proto"
)

const speedtestTimeoutDuration = 75 * time.Second

func (s *Server) handleInternetSpeedTest(w http.ResponseWriter, r *http.Request) {
	ac, ok := s.resolveConnectedNode(w, r.PathValue("id"))
	if !ok {
		return
	}

	result, err := ac.sendWithTimeout(proto.Command{
		ID:     newCommandID(),
		Action: "network.speedtest",
	}, speedtestTimeoutDuration)
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
