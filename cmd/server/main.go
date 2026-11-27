package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/leowmjw/go-temporal-workshop-2026/internal/workshop"
	"github.com/starfederation/datastar-go/datastar"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
)

//go:embed static/*
var staticFiles embed.FS

type server struct {
	client      client.Client
	namespace   string
	taskQueue   string
	nexusClient *nexusClients
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

	s := &server{client: c, namespace: namespace, taskQueue: taskQueue, nexusClient: &nexusClients{address: address, byNS: map[string]client.Client{}}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.home)
	mux.HandleFunc("GET /scenario/nexus", s.nexusPage)
	mux.HandleFunc("GET /scenario/{name}", s.scenario)
	mux.HandleFunc("GET /health", s.health)
	mux.Handle("GET /static/", http.FileServer(http.FS(staticFiles)))
	mux.HandleFunc("POST /api/update", s.update)
	mux.HandleFunc("POST /api/standalone", s.standalone)
	mux.HandleFunc("GET /api/forge/watch", s.forgeWatch)
	mux.HandleFunc("POST /api/forge/{action}", s.forgeControl)
	mux.HandleFunc("GET /api/nexus/status", s.nexusStatus)
	mux.HandleFunc("POST /api/nexus/payment", s.nexusPayment)
	mux.HandleFunc("GET /api/nexus/teamb/runs", s.nexusTeamBRuns)
	mux.HandleFunc("GET /api/snapshot", s.snapshot)

	httpAddress := env("WORKSHOP_HTTP_ADDR", "0.0.0.0:8080")
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
	if name == "standalone" {
		writeHTML(w, standalonePage(info.title, info.status, info.intro))
		return
	}
	if name == "update-with-start" {
		action = `<button data-on:click="@post('/api/update')">Apply +1 update</button><div id="result"></div>`
	}
	writeHTML(w, `<!doctype html><html><head><meta charset="utf-8"><title>`+info.title+`</title><script type="module" src="/static/datastar.js"></script></head><body style="font:16px system-ui;max-width:900px;margin:auto;padding:2rem"><p><a href="/">← Overview</a></p><h1>`+info.title+`</h1><p><strong>`+info.status+`</strong></p><p>`+info.intro+`</p>`+action+`</body></html>`)
}

// standalonePage renders the standalone-activity lifecycle playground. It seeds
// a `forge` signal so the same shape arrives from the SSE stream and from
// lifecycle control responses.
func standalonePage(title, status, intro string) string {
	var options strings.Builder
	for _, quote := range workshop.Quotes {
		options.WriteString(`<option value="` + strconv.Itoa(quote.ID) + `">` + html.EscapeString(quote.Text) + ` — ` + html.EscapeString(quote.Author) + `</option>`)
	}
	seed := `{"quoteId":1,"activityId":"","status":"IDLE","forge":{"quoteId":1,"status":"IDLE","running":false,"attempt":0,"heartbeats":0,"step":0,"steps":16,"stage":"","activityId":"","quote":null,"error":""}}`

	return `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width">
<title>` + title + `</title><script type="module" src="/static/datastar.js"></script>
<style>
 body{font:16px system-ui;max-width:900px;margin:auto;padding:2rem;color:#111}
 .panel{border:1px solid #ddd;border-radius:.5rem;padding:1rem;margin:1rem 0}
 .bar{height:10px;background:#eee;border-radius:5px;overflow:hidden;margin:.6rem 0}
 .fill{height:100%;background:#2f6feb;transition:width .3s ease}
 .row{display:flex;gap:1.5rem;flex-wrap:wrap;font-variant-numeric:tabular-nums}
 .k{color:#666;font-size:.8rem;text-transform:uppercase;letter-spacing:.04em}
 button{font:inherit;padding:.45rem .9rem;border-radius:.4rem;border:1px solid #bbb;background:#fff;cursor:pointer}
 button:hover:not(:disabled){background:#f3f3f3}
 button:disabled{opacity:.4;cursor:not-allowed}
 .danger{border-color:#c33;color:#c33}
 blockquote{margin:1rem 0;padding-left:1rem;border-left:3px solid #2f6feb;font-size:1.05rem}
 select{font:inherit;padding:.4rem;max-width:100%}
 .pill{display:inline-block;padding:.15rem .5rem;border-radius:1rem;font-size:.75rem;font-weight:600}
 .RUNNING{background:#e3f0ff;color:#1a56b8}.PAUSED{background:#fff4d1;color:#8a6100}
 .COMPLETED{background:#e2f7e5;color:#1c6b2b}.CANCELED,.TERMINATED,.FAILED,.TIMED_OUT{background:#fde8e8;color:#a11}
</style></head><body>
<p><a href="/">← Overview</a></p>
<h1>` + title + `</h1><p><strong>` + status + `</strong></p><p>` + intro + `</p>
<div class="panel" data-signals="` + html.EscapeString(seed) + `">
 <label class="k" for="quote">Pick a quote</label><br>
 <select id="quote" data-bind="quoteId">` + options.String() + `</select>
 <button data-on:click="@get('/api/forge/watch?quoteId=' + $quoteId)">Forge it</button>
 <p class="k">The calculator starts a durable activity. Every step is a heartbeat, so pause, cancel, and terminate are real Temporal RPCs — not UI state.</p>
</div>
<div class="panel">
 <div class="row">
  <div><div class="k">Status</div><span class="pill" data-class="{'RUNNING':$forge.status==='RUNNING','PAUSED':$forge.status==='PAUSED','COMPLETED':$forge.status==='COMPLETED','CANCELED':$forge.status==='CANCELED','TERMINATED':$forge.status==='TERMINATED','FAILED':$forge.status==='FAILED','TIMED_OUT':$forge.status==='TIMED_OUT'}" data-text="$forge.status"></span></div>
  <div><div class="k">Step</div><span data-text="$forge.step + ' / ' + $forge.steps"></span></div>
  <div><div class="k">Attempt</div><span data-text="$forge.attempt"></span></div>
  <div><div class="k">Heartbeats</div><span data-text="$forge.heartbeats"></span></div>
 </div>
 <div class="bar"><div class="fill" data-attr="style: 'width:' + ($forge.steps ? ($forge.step / $forge.steps * 100) : 0) + '%'"></div></div>
 <p><span class="k">Stage</span> <span data-text="$forge.stage"></span></p>
 <p data-show="$forge.activityId !== ''"><span class="k">Activity ID</span> <code data-text="$forge.activityId"></code></p>
 <p data-show="$forge.error !== ''" style="color:#a11" data-text="$forge.error"></p>
 <div class="row">
  <button data-on:click="@post('/api/forge/pause')" data-attr="disabled: $forge.status !== 'RUNNING'">Pause</button>
  <button data-on:click="@post('/api/forge/unpause')" data-attr="disabled: $forge.status !== 'PAUSED'">Unpause</button>
  <button class="danger" data-on:click="@post('/api/forge/cancel')" data-attr="disabled: $forge.status !== 'RUNNING' && $forge.status !== 'PAUSED'">Cancel</button>
  <button class="danger" data-on:click="@post('/api/forge/terminate')" data-attr="disabled: $forge.status !== 'RUNNING' && $forge.status !== 'PAUSED'">Terminate</button>
 </div>
 <blockquote data-show="$forge.quote !== null"><span data-text="$forge.quote ? $forge.quote.text : ''"></span> — <em data-text="$forge.quote ? $forge.quote.author : ''"></em></blockquote>
</div>
</body></html>`
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

type forgeControlRequest struct {
	ActivityID string `json:"activityId"`
}

// forgeControl drives the lifecycle RPCs that exist only for standalone
// activities: pause, unpause, cancel, and terminate an activity by ID.
func (s *server) forgeControl(w http.ResponseWriter, r *http.Request) {
	if s.client == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "Temporal server is not running"})
		return
	}
	var request forgeControlRequest
	if err := datastar.ReadSignals(r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if request.ActivityID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "activityId is required"})
		return
	}

	action := r.PathValue("action")
	if action != "pause" && action != "unpause" && action != "cancel" && action != "terminate" {
		writeJSON(w, http.StatusNotFound, map[string]any{"forgeError": "unknown action: " + action})
		return
	}

	handle := s.client.GetActivityHandle(client.GetActivityHandleOptions{ActivityID: request.ActivityID})
	var err error
	switch action {
	case "pause":
		err = handle.Pause(r.Context(), client.PauseActivityOptions{Reason: "operator paused the forge"})
	case "unpause":
		err = handle.Unpause(r.Context(), client.UnpauseActivityOptions{Reason: "operator resumed the forge"})
	case "cancel":
		err = handle.Cancel(r.Context(), client.CancelActivityOptions{Reason: "operator canceled the forge"})
	case "terminate":
		err = handle.Terminate(r.Context(), client.TerminateActivityOptions{Reason: "operator terminated the forge"})
	}
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"forgeError": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"forgeError": "", "lastAction": action})
}

// forgeWatch starts a forge activity for the requested quote and streams its
// state to the browser over SSE until it reaches a terminal state. Starting and
// watching in one request keeps the page stateless.
func (s *server) forgeWatch(w http.ResponseWriter, r *http.Request) {
	if s.client == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "Temporal server is not running"})
		return
	}
	quoteID, _ := strconv.Atoi(r.URL.Query().Get("quoteId"))
	quote, ok := workshop.QuoteByID(quoteID)
	if !ok {
		quote = workshop.Quotes[0]
		quoteID = quote.ID
	}

	activityID := fmt.Sprintf("quote-forge-%d-%d", quoteID, time.Now().UnixNano())
	if _, err := s.client.ExecuteActivity(r.Context(), client.StartActivityOptions{
		ID:                     activityID,
		TaskQueue:              s.taskQueue,
		ScheduleToCloseTimeout: 5 * time.Minute,
		StartToCloseTimeout:    time.Minute,
		HeartbeatTimeout:       2 * time.Second,
		RetryPolicy:            &temporal.RetryPolicy{MaximumAttempts: 3},
	}, workshop.ForgeQuoteActivity, quoteID); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}

	handle := s.client.GetActivityHandle(client.GetActivityHandleOptions{ActivityID: activityID})
	sse := datastar.NewSSE(w, r)
	ticker := time.NewTicker(400 * time.Millisecond)
	defer ticker.Stop()

	for {
		status, terminal := s.describeForge(r.Context(), handle, activityID, quote)
		if err := sse.MarshalAndPatchSignals(map[string]any{"forge": status, "activityId": activityID}); err != nil {
			return
		}
		if terminal || sse.IsClosed() {
			return
		}
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *server) describeForge(ctx context.Context, handle client.ActivityHandle, activityID string, quote workshop.Quote) (map[string]any, bool) {
	status := map[string]any{
		"activityId": activityID,
		"status":     "RUNNING",
		"running":    true,
		"attempt":    1,
		"heartbeats": 0,
		"step":       0,
		"steps":      workshop.ForgeSteps,
		"stage":      workshop.ForgeStages[0],
		"quoteId":    quote.ID,
		"error":      "",
	}
	description, err := handle.Describe(ctx, client.DescribeActivityOptions{
		IncludeHeartbeatDetails: true,
		IncludeOutcome:          true,
	})
	if err != nil {
		status["error"] = err.Error()
		return status, true
	}

	status["status"] = strings.ToUpper(description.Status.String())
	status["attempt"] = description.Attempt
	status["heartbeats"] = description.TotalHeartbeatCount

	var progress workshop.ForgeProgress
	if err := description.GetHeartbeatDetails(&progress); err == nil {
		status["step"] = progress.Step
		status["steps"] = progress.Steps
		status["stage"] = progress.Stage
	}
	var result workshop.Quote
	if err := description.GetResult(&result); err == nil {
		status["quote"] = result
	}
	if description.HasOutcomeFailure() {
		status["error"] = description.RawResponse.GetOutcome().GetFailure().GetMessage()
	}

	terminal := description.Status != enumspb.ACTIVITY_EXECUTION_STATUS_RUNNING &&
		description.Status != enumspb.ACTIVITY_EXECUTION_STATUS_PAUSED
	status["running"] = !terminal
	return status, terminal
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
