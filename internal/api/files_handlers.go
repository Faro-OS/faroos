package api

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"path"

	"github.com/faroos/faroos/internal/fileops"
	"github.com/faroos/faroos/internal/proto"
)

func (s *Server) handleListFiles(w http.ResponseWriter, r *http.Request) {
	ac, ok := s.resolveConnectedNode(w, r.PathValue("id"))
	if !ok {
		return
	}
	params, _ := json.Marshal(pathParams{Path: r.URL.Query().Get("path")})

	result, err := ac.send(proto.Command{ID: newCommandID(), Action: "files.list", Params: params})
	if err != nil {
		http.Error(w, err.Error(), http.StatusGatewayTimeout)
		return
	}
	if !result.OK {
		http.Error(w, result.Error, http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if len(result.Result) == 0 {
		w.Write([]byte("[]"))
		return
	}
	w.Write(result.Result)
}

func (s *Server) handleDownloadFile(w http.ResponseWriter, r *http.Request) {
	ac, ok := s.resolveConnectedNode(w, r.PathValue("id"))
	if !ok {
		return
	}
	filePath := r.URL.Query().Get("path")
	params, _ := json.Marshal(pathParams{Path: filePath})

	result, err := ac.send(proto.Command{ID: newCommandID(), Action: "files.download", Params: params})
	if err != nil {
		http.Error(w, err.Error(), http.StatusGatewayTimeout)
		return
	}
	if !result.OK {
		http.Error(w, result.Error, http.StatusBadGateway)
		return
	}

	var payload struct {
		ContentB64 string `json:"contentB64"`
	}
	if err := json.Unmarshal(result.Result, &payload); err != nil {
		http.Error(w, "malformed agent response", http.StatusBadGateway)
		return
	}
	data, err := base64.StdEncoding.DecodeString(payload.ContentB64)
	if err != nil {
		http.Error(w, "malformed file content from agent", http.StatusBadGateway)
		return
	}

	filename := path.Base(filePath)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+url.PathEscape(filename)+`"`)
	w.Write(data)
}

func (s *Server) handleUploadFile(w http.ResponseWriter, r *http.Request) {
	ac, ok := s.resolveConnectedNode(w, r.PathValue("id"))
	if !ok {
		return
	}
	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		http.Error(w, "path query parameter is required", http.StatusBadRequest)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, fileops.MaxTransferSize)
	data, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "file too large or unreadable", http.StatusRequestEntityTooLarge)
		return
	}

	params, _ := json.Marshal(writeFileParams{Path: filePath, ContentB64: base64.StdEncoding.EncodeToString(data)})
	result, err := ac.send(proto.Command{ID: newCommandID(), Action: "files.upload", Params: params})
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

func (s *Server) handleDeleteFile(w http.ResponseWriter, r *http.Request) {
	ac, ok := s.resolveConnectedNode(w, r.PathValue("id"))
	if !ok {
		return
	}
	params, _ := json.Marshal(pathParams{Path: r.URL.Query().Get("path")})

	result, err := ac.send(proto.Command{ID: newCommandID(), Action: "files.delete", Params: params})
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

func (s *Server) handleCreateDirectory(w http.ResponseWriter, r *http.Request) {
	ac, ok := s.resolveConnectedNode(w, r.PathValue("id"))
	if !ok {
		return
	}
	directoryPath := r.URL.Query().Get("path")
	if directoryPath == "" || directoryPath == "/" {
		http.Error(w, "directory path is required", http.StatusBadRequest)
		return
	}
	params, _ := json.Marshal(pathParams{Path: directoryPath})
	result, err := ac.send(proto.Command{ID: newCommandID(), Action: "files.mkdir", Params: params})
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

func (s *Server) handleRenameFile(w http.ResponseWriter, r *http.Request) {
	ac, ok := s.resolveConnectedNode(w, r.PathValue("id"))
	if !ok {
		return
	}
	var params renameFileParams
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil || params.From == "" || params.To == "" {
		http.Error(w, "from and to paths are required", http.StatusBadRequest)
		return
	}
	raw, _ := json.Marshal(params)
	result, err := ac.send(proto.Command{ID: newCommandID(), Action: "files.rename", Params: raw})
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

type pathParams struct {
	Path string `json:"path"`
}

type writeFileParams struct {
	Path       string `json:"path"`
	ContentB64 string `json:"contentB64"`
}

type renameFileParams struct {
	From string `json:"from"`
	To   string `json:"to"`
}
