package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/leowmjw/go-temporal-workshop-2026/internal/workshop"
	"github.com/starfederation/datastar-go/datastar"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
)

//go:embed static/*
var staticFiles embed.FS

type server struct {
	client    client.Client
	namespace string
	taskQueue string
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	namespace := env("TEMPORAL_NAMESPACE", "default")
	taskQueue := env("TEMPORAL_TASK_QUEUE", "workshop-main")
	address := env("TEMPORAL_ADDRESS", "127.0.0.1:7233")

	c, err := client.Dial(client.Options{HostPort: address, Namespace: namespace})
	if err != nil {
		slog.Warn("Temporal unavailable; the UI will use preview mode", "address", address, "error", err)
		c = nil
	}
	if c != nil {
		defer c.Close()
	}

	s := &server{client: c, namespace: namespace, taskQueue: taskQueue}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.home)
	mux.HandleFunc("GET /scenario/{name}", s.scenario)
	mux.HandleFunc("GET /health", s.health)
	mux.Handle("GET /static/", http.FileServer(http.FS(staticFiles)))
	mux.HandleFunc("POST /api/update", s.update)
	mux.HandleFunc("POST /api/standalone", s.standalone)
	mux.HandleFunc("GET /api/snapshot", s.snapshot)

	httpAddress := env("WORKSHOP_HTTP_ADDR", "127.0.0.1:8080")
	slog.Info("workshop server started", "address", httpAddress)
	if err := http.ListenAndServe(httpAddress, mux); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func (s *server) home(w http.ResponseWriter, r *http.Request) {
	writeHTML(w, `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width">
<title>Temporal Go Workshop 2025–2026</title><script type="module" src="/static/datastar.js"></script>
<style>body{font-family:system-ui;margin:0;display:grid;grid-template-columns:260px 1fr}nav{padding:1rem;border-right:1px solid #ddd}main{padding:1.5rem}a{display:block;margin:.6rem 0}.tag{font-size:.8rem;color:#555}.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(220px,1fr));gap:1rem}.card{border:1px solid #ddd;border-radius:.5rem;padding:1rem}</style></head><body>
<nav><h2>Go workshop</h2><a href="/">Overview</a><a href="/scenario/update-with-start">Update-With-Start</a><a href="/scenario/standalone">Standalone Activities</a><a href="/scenario/nexus">Nexus</a><a href="/scenario/dynamic">Dynamic Workflows</a><a href="/scenario/external-storage">External Storage</a><a href="/scenario/worker-versioning">Worker Versioning</a><a href="/scenario/cloud">Cloud roadmap</a></nav>
<main><h1>New Temporal capabilities for Go, 2025–2026</h1><p>Each local demo replaces an older, more complex pattern.</p>
<div class="grid">
<div class="card"><h3><a href="/scenario/update-with-start">Update-With-Start</a></h3><p class="tag">Server GA · June 2025</p><p>Start-or-update + wait for an accepted result in one round trip.</p></div>
<div class="card"><h3><a href="/scenario/standalone">Standalone Activities</a></h3><p class="tag">Go SDK GA · September 2026</p><p>Durable standalone jobs without a wrapper workflow.</p></div>
<div class="card"><h3><a href="/scenario/nexus">Nexus</a></h3><p class="tag">GA · March 2025</p><p>Typed cross-namespace service operations.</p></div>
<div class="card"><h3><a href="/scenario/dynamic">Dynamic registration</a></h3><p class="tag">Go SDK · July 2025</p><p>Register fallback workflows and activities by name.</p></div>
<div class="card"><h3><a href="/scenario/external-storage">External Storage</a></h3><p class="tag">Preview · 2026</p><p>Payload claim-checks avoid large Event History records.</p></div>
<div class="card"><h3><a href="/scenario/worker-versioning">Worker Versioning</a></h3><p class="tag">Preview · 2025</p><p>Pin or route workflow runs across worker deployments.</p></div>
</div></main></body></html>`)
}

func (s *server) scenario(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	titles := map[string]struct {
		title, intro, status string
	}{
		"update-with-start": {"Update-With-Start", "Create or update workflow state in one idempotent API call. This replaces start-then-update polling and an outbox for workflow-led synchronous APIs.", "Server GA · June 2025"},
		"standalone":        {"Standalone Activities", "Submit durable work directly to the client. No wrapper workflow or custom job table is needed.", "Go SDK GA · September 2026"},
		"nexus":             {"Nexus", "Call typed operations across namespaces or teams. Full Nexus requires endpoint configuration; this workshop documents the Go setup and boundary.", "GA · March 2025"},
		"dynamic":           {"Dynamic Workflows and Activities", "A worker can accept named fallback handlers instead of a generated switch of wrapper functions.", "Go SDK · July 2025"},
		"external-storage":  {"External Storage / Claim-Check", "Store large payloads outside Event History and pass a reference through Temporal.", "Public preview · 2026"},
		"worker-versioning": {"Worker Versioning", "Deployment-aware routing and pinned workflow versions simplify safe code replacement.", "Preview · Replay 2025"},
		"cloud":             {"Cloud-only roadmap", "Future advanced demos: Multi-region / same-region replication, SCIM, API keys, Serverless Workers, and managed failover.", "Future advanced demo"},
	}
	info, ok := titles[name]
	if !ok {
		http.NotFound(w, r)
		return
	}
	action := ""
	if name == "update-with-start" {
		action = `<button data-on:click="@post('/api/update')">Apply +1 update</button><div id="result"></div>`
	}
	if name == "standalone" {
		action = `<button data-on:click="@post('/api/standalone')">Submit standalone activity</button><div id="result"></div>`
	}
	writeHTML(w, `<!doctype html><html><head><meta charset="utf-8"><title>`+info.title+`</title><script type="module" src="/static/datastar.js"></script></head><body style="font:16px system-ui;max-width:900px;margin:auto;padding:2rem"><p><a href="/">← Overview</a></p><h1>`+info.title+`</h1><p><strong>`+info.status+`</strong></p><p>`+info.intro+`</p>`+action+`</body></html>`)
}

func (s *server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "temporal": s.client != nil, "namespace": s.namespace, "taskQueue": s.taskQueue})
}

func (s *server) update(w http.ResponseWriter, r *http.Request) {
	if s.client == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "Temporal server is not running"})
		return
	}
	id := r.FormValue("workflowId")
	if id == "" {
		id = "demo-counter"
	}
	operation := s.client.NewWithStartWorkflowOperation(client.StartWorkflowOptions{
		ID: id, TaskQueue: s.taskQueue,
		WorkflowExecutionTimeout: 10 * time.Minute,
		WorkflowIDConflictPolicy: enumspb.WORKFLOW_ID_CONFLICT_POLICY_USE_EXISTING,
	}, workshop.StateWorkflow, 0)
	handle, err := s.client.UpdateWithStartWorkflow(r.Context(), client.UpdateWithStartWorkflowOptions{
		StartWorkflowOperation: operation,
		UpdateOptions: client.UpdateWorkflowOptions{
			UpdateName:   "Apply",
			Args:         []any{workshop.UpdateRequest{Delta: 1}},
			WaitForStage: client.WorkflowUpdateStageCompleted,
		},
	})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	var result workshop.State
	if err := handle.Get(r.Context(), &result); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	if r.Header.Get("Accept") == "text/event-stream" || r.Header.Get("Datastar-Request") == "true" {
		sse := datastar.NewSSE(w, r)
		_ = sse.PatchElements(`<div id="result">Update accepted: counter ` + fmt.Sprint(result.Counter) + `</div>`)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"workflowId": id, "state": result})
}

func (s *server) standalone(w http.ResponseWriter, r *http.Request) {
	if s.client == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "Temporal server is not running"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	handle, err := s.client.ExecuteActivity(ctx, client.StartActivityOptions{
		ID: "standalone-" + time.Now().Format("150405.000"), TaskQueue: s.taskQueue,
		ScheduleToCloseTimeout: 10 * time.Second,
	}, workshop.StandaloneNotification, "DataStar audience")
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	var result string
	if err := handle.Get(ctx, &result); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	if r.Header.Get("Accept") == "text/event-stream" || r.Header.Get("Datastar-Request") == "true" {
		sse := datastar.NewSSE(w, r)
		_ = sse.PatchElements(`<div id="result">` + result + `</div>`)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"result": result})
}

func (s *server) snapshot(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("workflowId")
	if id == "" {
		id = "demo-counter"
	}
	if s.client == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "Temporal server is not running"})
		return
	}
	encoded, err := s.client.QueryWorkflow(r.Context(), id, "", "Snapshot")
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
		return
	}
	var state workshop.State
	if err := encoded.Get(&state); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, state)
}

func writeHTML(w http.ResponseWriter, r string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(r))
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
