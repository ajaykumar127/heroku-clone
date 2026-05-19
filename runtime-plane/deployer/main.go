package main

import (
	"encoding/json"
	"log"
	"net/http"
)

// scaleRequest is the payload for the /scale endpoint.
type scaleRequest struct {
	AppName  string `json:"app_name"`
	Replicas int    `json:"replicas"`
}

// destroyRequest is the payload for the /destroy endpoint.
type destroyRequest struct {
	AppName string `json:"app_name"`
}

// deployResponse is returned by /deploy, /scale, and /destroy.
type deployResponse struct {
	Status string `json:"status"`
	Output string `json:"output"`
}

func main() {
	cfg := LoadConfig()

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/deploy", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req DeployRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}
		if req.AppName == "" || req.Image == "" {
			http.Error(w, "app_name and image are required", http.StatusBadRequest)
			return
		}
		output, err := Deploy(cfg, req)
		resp := deployResponse{Output: output}
		if err != nil {
			resp.Status = "failed"
			resp.Output += "\nError: " + err.Error()
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			resp.Status = "succeeded"
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	mux.HandleFunc("/scale", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req scaleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}
		if req.AppName == "" {
			http.Error(w, "app_name is required", http.StatusBadRequest)
			return
		}
		output, err := Scale(cfg, req.AppName, req.Replicas)
		resp := deployResponse{Output: output}
		if err != nil {
			resp.Status = "failed"
			resp.Output += "\nError: " + err.Error()
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			resp.Status = "succeeded"
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	mux.HandleFunc("/destroy", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req destroyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}
		if req.AppName == "" {
			http.Error(w, "app_name is required", http.StatusBadRequest)
			return
		}
		output, err := Destroy(cfg, req.AppName)
		resp := deployResponse{Output: output}
		if err != nil {
			resp.Status = "failed"
			resp.Output += "\nError: " + err.Error()
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			resp.Status = "succeeded"
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	addr := ":" + cfg.Port
	log.Printf("deployer listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
