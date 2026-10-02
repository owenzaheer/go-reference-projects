package server

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"portfolio.local/reference/internal/workflow"
	"strings"
	"time"
)

func Run(defaultConfig string) {
	project := flag.String("project", defaultConfig, "path to project.json")
	addr := flag.String("addr", "127.0.0.1:8000", "listen address")
	flag.Parse()
	bytes, err := os.ReadFile(*project)
	if err != nil {
		log.Fatal(err)
	}
	var config struct {
		ID      string   `json:"id"`
		Actions []string `json:"actions"`
	}
	if err = json.Unmarshal(bytes, &config); err != nil {
		log.Fatal(err)
	}
	e := workflow.New()
	mux := http.NewServeMux()
	reply := func(w http.ResponseWriter, status int, value any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(value)
	}
	mux.HandleFunc("GET /api/config", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(bytes)
	})
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		reply(w, 200, map[string]string{"status": "ok", "project": config.ID, "storage": "in-memory synthetic fixture"})
	})
	mux.HandleFunc("GET /api/state", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer local-operator" {
			reply(w, 403, map[string]string{"detail": "Operator required"})
			return
		}
		reply(w, 200, e.State())
	})
	mux.HandleFunc("POST /api/action", func(w http.ResponseWriter, r *http.Request) {
		roles := map[string]string{"Bearer local-learner": "learner", "Bearer local-operator": "operator", "Bearer local-instructor": "instructor", "Bearer local-clinic-a": "clinic-a", "Bearer local-clinic-b": "clinic-b"}
		role, ok := roles[r.Header.Get("Authorization")]
		if !ok {
			reply(w, 401, map[string]string{"detail": "Explicit local fixture role token required"})
			return
		}
		var cmd struct {
			Action  string          `json:"action"`
			Payload workflow.Record `json:"payload"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&cmd); err != nil || cmd.Payload == nil {
			reply(w, 422, map[string]string{"detail": "Valid command object required"})
			return
		}
		allowed := false
		for _, a := range config.Actions {
			if a == cmd.Action {
				allowed = true
			}
		}
		if !allowed {
			reply(w, 404, map[string]string{"detail": "Action unavailable in this project"})
			return
		}
		result, err := e.Command(cmd.Action, cmd.Payload, role)
		if err != nil {
			var p workflow.Problem
			if errors.As(err, &p) {
				reply(w, p.Status, map[string]string{"detail": p.Message})
				return
			}
			reply(w, 500, map[string]string{"detail": "Unexpected workflow error"})
			return
		}
		reply(w, 200, result)
	})
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		ui, err := os.ReadFile("../ui/console.html")
		if err != nil {
			reply(w, 500, map[string]string{"detail": "Run from the go family folder"})
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(strings.ReplaceAll(string(ui), "__CONFIG__", string(bytes))))
	})
	service := &http.Server{Addr: *addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	go func() {
		<-ctx.Done()
		deadline, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		service.Shutdown(deadline)
	}()
	log.Println("Local demo at http://" + *addr)
	if err := service.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
