package main

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/leowmjw/go-temporal-workshop-2026/internal/workshop"
	"github.com/starfederation/datastar-go/datastar"
	"go.temporal.io/api/operatorservice/v1"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
)

const setupNexusReminder = "Nexus is not wired yet. Run `mise run setup-nexus` to create namespaces team-a/team-b and the workshop-user-directory endpoint, then restart the worker."

type nexusWiring struct {
	Temporal bool `json:"temporal"`
	TeamA    bool `json:"teamA"`
	TeamB    bool `json:"teamB"`
	Endpoint bool `json:"endpoint"`
}

func (w nexusWiring) Wired() bool { return w.Temporal && w.TeamA && w.TeamB && w.Endpoint }

func (w nexusWiring) missing() []string {
	var out []string
	if !w.TeamA {
		out = append(out, "namespace "+workshop.TeamANamespace)
	}
	if !w.TeamB {
		out = append(out, "namespace "+workshop.TeamBNamespace)
	}
	if !w.Endpoint {
		out = append(out, "endpoint "+workshop.NexusEndpointName)
	}
	return out
}

type nexusClients struct {
	mu      sync.Mutex
	address string
	byNS    map[string]client.Client
}

func (n *nexusClients) forNamespace(namespace string) (client.Client, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if c, ok := n.byNS[namespace]; ok {
		return c, nil
	}
	c, err := client.Dial(client.Options{HostPort: n.address, Namespace: namespace})
	if err != nil {
		return nil, err
	}
	n.byNS[namespace] = c
	return c, nil
}

func (s *server) checkNexusWiring(ctx context.Context) nexusWiring {
	wiring := nexusWiring{Temporal: s.client != nil}
	if !wiring.Temporal {
		return wiring
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	for _, probe := range []struct {
		namespace string
		mark      *bool
	}{
		{workshop.TeamANamespace, &wiring.TeamA},
		{workshop.TeamBNamespace, &wiring.TeamB},
	} {
		_, err := s.client.WorkflowService().DescribeNamespace(ctx, &workflowservice.DescribeNamespaceRequest{Namespace: probe.namespace})
		*probe.mark = err == nil
	}
	endpoints, err := s.client.OperatorService().ListNexusEndpoints(ctx, &operatorservice.ListNexusEndpointsRequest{PageSize: 100})
	if err == nil {
		for _, endpoint := range endpoints.GetEndpoints() {
			if endpoint.GetSpec().GetName() == workshop.NexusEndpointName {
				wiring.Endpoint = true
			}
		}
	}
	return wiring
}

func (s *server) nexusPage(w http.ResponseWriter, r *http.Request) {
	role := "teama"
	if r.URL.Query().Get("role") == "teamb" {
		role = "teamb"
	}
	writeHTML(w, `<!doctype html><html><head><meta charset="utf-8"><title>Nexus — cross-team operations</title><script type="module" src="/static/datastar.js"></script>
<style>
body{font:16px system-ui;max-width:960px;margin:auto;padding:2rem}
.roles{display:flex;gap:.5rem;margin:1rem 0}
.roles button{padding:.5rem 1rem;border:1px solid #999;border-radius:.5rem;background:#fff;cursor:pointer}
.roles button.active{background:#24292f;color:#fff;border-color:#24292f}
.panel{border:1px solid #ddd;border-radius:.5rem;padding:1rem;margin-top:1rem}
.wired{border-left:4px solid #1a7f37}
.notwired{border-left:4px solid #b35900;background:#fff8ec}
code{background:#f3f3f3;padding:.1rem .3rem;border-radius:.25rem}
input{padding:.4rem;border:1px solid #bbb;border-radius:.25rem;margin-right:.5rem}
button{padding:.45rem .9rem;border:1px solid #24292f;border-radius:.25rem;background:#24292f;color:#fff;cursor:pointer}
button:disabled{opacity:.4;cursor:not-allowed}
.owns{font-size:.9rem;color:#444;margin:.25rem 0}
.run{font-family:ui-monospace,monospace;font-size:.85rem;padding:.15rem 0;border-bottom:1px dashed #eee}
ol.trace li{margin:.3rem 0}
.muted{color:#666;font-size:.85rem}
</style></head><body>
<div data-signals="{role:'`+role+`',wired:false,userid:'u-101',amount:42}">
<p><a href="/">← Overview</a></p>
<h1>Nexus: TeamA pays, TeamB answers</h1>
<p><strong>GA · March 2025</strong> · Typed cross-namespace operations replace custom RPC, duplicated routing, and ad-hoc callbacks.</p>
<div id="wiring" data-init="@get('/api/nexus/status')">checking Nexus wiring…</div>
<div class="roles">
<button data-class:active="$role==='teama'" data-on:click="$role='teama'">TeamA (caller)</button>
<button data-class:active="$role==='teamb'" data-on:click="$role='teamb'">TeamB (handler)</button>
<span class="muted" style="align-self:center">switch roles to see what each team owns — open a second tab with the other role to watch a call cross the boundary</span>
</div>

<div class="panel" data-show="$role==='teama'">
<h2>TeamA · namespace <code>`+workshop.TeamANamespace+`</code></h2>
<p class="owns">Owns workflow: <strong>PAYMENT</strong> (<code>PaymentWorkflow</code> on task queue <code>`+workshop.TeamATaskQueue+`</code>)</p>
<p>TeamA must not touch TeamB's database. It makes two typed calls through endpoint <code>`+workshop.NexusEndpointName+`</code>: a quick <code>`+workshop.NexusGetUserOp+`</code> lookup, then <code>`+workshop.NexusReviewCreditOp+`</code> — a <em>durable promise</em>: TeamB runs a real multi-stage review workflow and TeamA simply awaits the verdict, even if TeamB restarts mid-review.</p>
<p>
<label>user <input data-bind:userid size="8"></label>
<label>credits <input data-bind:amount type="number" size="5" min="1"></label>
<button data-attr:disabled="$wired!==true" data-on:click="@post('/api/nexus/payment')">Charge user over Nexus</button>
</p>
<p class="muted">TeamB's workshop credit limit is `+"250"+` — charge more than that to watch the promise come back <strong>declined</strong>. The review takes a few seconds; switch to the TeamB tab to see it run.</p>
<p class="muted" data-show="$wired!==true">The button unlocks once the wiring check above passes.</p>
<div id="receipt"></div>
</div>

<div class="panel" data-show="$role==='teamb'">
<h2>TeamB · namespace <code>`+workshop.TeamBNamespace+`</code></h2>
<p class="owns">Owns workflows: <strong>USER</strong> (<code>UserWorkflow</code>) and <strong>CREDIT&nbsp;REVIEW</strong> (<code>CreditReviewWorkflow</code>) on task queue <code>`+workshop.TeamBTaskQueue+`</code></p>
<p class="owns">Serves Nexus service <code>`+workshop.NexusServiceName+`</code> · <code>`+workshop.NexusGetUserOp+`</code> → <code>UserWorkflow</code> (quick lookup) · <code>`+workshop.NexusReviewCreditOp+`</code> → <code>CreditReviewWorkflow</code> (long-running review, queryable stage, idempotent run ID)</p>
<p>TeamB never exposes its database or internals — only this service contract. Every call from TeamA appears below within a few seconds (polling):</p>
<div data-on-interval__duration.3s="@get('/api/nexus/teamb/runs')">
<div id="teamb-runs"><p class="muted">no handler runs seen yet</p></div>
</div>
</div>
</div></body></html>`)
}

func (s *server) nexusStatus(w http.ResponseWriter, r *http.Request) {
	wiring := s.checkNexusWiring(r.Context())
	if isDatastar(r) {
		sse := datastar.NewSSE(w, r)
		var card string
		if wiring.Wired() {
			card = `<div id="wiring" class="panel wired">Wiring OK · namespaces <code>` + workshop.TeamANamespace + `</code> + <code>` + workshop.TeamBNamespace + `</code> · endpoint <code>` + workshop.NexusEndpointName + `</code> → <code>` + workshop.TeamBNamespace + `/` + workshop.TeamBTaskQueue + `</code></div>`
		} else {
			card = `<div id="wiring" class="panel notwired"><strong>Demo not wired.</strong> Missing: ` + html.EscapeString(strings.Join(wiring.missing(), ", ")) + `.<br>` + html.EscapeString(setupNexusReminder) + `</div>`
		}
		_ = sse.PatchElements(card)
		_ = sse.MarshalAndPatchSignals(map[string]any{"wired": wiring.Wired()})
		return
	}
	status := http.StatusOK
	if !wiring.Wired() {
		status = http.StatusServiceUnavailable
	}
	writeJSON(w, status, map[string]any{"wired": wiring.Wired(), "checks": wiring, "hint": setupNexusReminder})
}

func (s *server) nexusPayment(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	wiring := s.checkNexusWiring(ctx)
	if !wiring.Wired() {
		s.refuseNexus(w, r, wiring)
		return
	}
	request := workshop.PaymentRequest{UserID: "u-101", Amount: 42}
	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		var signals struct {
			UserID string `json:"userid"`
			Amount any    `json:"amount"`
		}
		if err := datastar.ReadSignals(r, &signals); err == nil {
			if signals.UserID != "" {
				request.UserID = signals.UserID
			}
			request.Amount = coerceAmount(signals.Amount, request.Amount)
		}
	} else {
		if value := r.FormValue("userid"); value != "" {
			request.UserID = value
		}
		if value := r.FormValue("amount"); value != "" {
			if parsed, err := strconv.Atoi(value); err == nil {
				request.Amount = parsed
			}
		}
	}

	teamA, err := s.nexusClient.forNamespace(workshop.TeamANamespace)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	run, err := teamA.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:        fmt.Sprintf("payment-%s-%d", request.UserID, time.Now().Unix()),
		TaskQueue: workshop.TeamATaskQueue,
	}, workshop.PaymentWorkflow, request)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	var receipt workshop.PaymentReceipt
	if err := run.Get(ctx, &receipt); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	if isDatastar(r) {
		sse := datastar.NewSSE(w, r)
		var lines strings.Builder
		for _, step := range receipt.Trace {
			lines.WriteString(`<li>` + html.EscapeString(step) + `</li>`)
		}
		cardClass, headline := "wired", "Charged"
		if receipt.Status != "charged" {
			cardClass, headline = "notwired", "Declined"
		}
		_ = sse.PatchElements(`<div id="receipt" class="panel ` + cardClass + `"><h3>Receipt · ` + html.EscapeString(receipt.PaymentID) + `</h3><p>` + headline + ` <strong>` + strconv.Itoa(receipt.Amount) + `</strong> credits · user <code>` + html.EscapeString(receipt.UserID) + `</code> — status ` + receipt.Status + `</p><ol class="trace">` + lines.String() + `</ol><p class="muted">The lookup and the multi-second credit review both crossed the namespace boundary through Nexus; TeamA never dialed TeamB directly.</p></div>`)
		return
	}
	writeJSON(w, http.StatusOK, receipt)
}

func (s *server) nexusTeamBRuns(w http.ResponseWriter, r *http.Request) {
	wiring := s.checkNexusWiring(r.Context())
	if !wiring.Wired() {
		if isDatastar(r) {
			sse := datastar.NewSSE(w, r)
			_ = sse.PatchElements(`<div id="teamb-runs"><p class="muted">` + html.EscapeString(setupNexusReminder) + `</p></div>`)
			return
		}
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": setupNexusReminder, "checks": wiring})
		return
	}
	teamB, err := s.nexusClient.forNamespace(workshop.TeamBNamespace)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	response, err := teamB.ListWorkflow(r.Context(), &workflowservice.ListWorkflowExecutionsRequest{
		Namespace: workshop.TeamBNamespace,
		PageSize:  8,
		Query:     "WorkflowType IN ('UserWorkflow', 'CreditReviewWorkflow')",
	})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	type runRow struct {
		ID      string `json:"id"`
		Status  string `json:"status"`
		Started string `json:"started"`
	}
	rows := make([]runRow, 0, len(response.GetExecutions()))
	for _, execution := range response.GetExecutions() {
		rows = append(rows, runRow{
			ID:      execution.GetExecution().GetWorkflowId(),
			Status:  strings.TrimPrefix(execution.GetStatus().String(), "WORKFLOW_EXECUTION_STATUS_"),
			Started: execution.GetStartTime().AsTime().Format("15:04:05"),
		})
	}
	if isDatastar(r) {
		sse := datastar.NewSSE(w, r)
		var list strings.Builder
		if len(rows) == 0 {
			list.WriteString(`<p class="muted">no handler runs seen yet — fire a payment from the TeamA view</p>`)
		}
		for _, row := range rows {
			list.WriteString(`<div class="run">` + html.EscapeString(row.ID) + ` · ` + row.Status + ` · ` + row.Started + `</div>`)
		}
		_ = sse.PatchElements(`<div id="teamb-runs">` + list.String() + `</div>`)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"runs": rows})
}

func (s *server) refuseNexus(w http.ResponseWriter, r *http.Request, wiring nexusWiring) {
	if isDatastar(r) {
		sse := datastar.NewSSE(w, r)
		_ = sse.PatchElements(`<div id="receipt" class="panel notwired"><strong>Refusing to run.</strong> ` + html.EscapeString(setupNexusReminder) + `<br><span class="muted">Missing: ` + html.EscapeString(strings.Join(wiring.missing(), ", ")) + `</span></div>`)
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": setupNexusReminder, "checks": wiring})
}

func isDatastar(r *http.Request) bool {
	return r.Header.Get("Accept") == "text/event-stream" || r.Header.Get("Datastar-Request") == "true"
}

func coerceAmount(value any, fallback int) int {
	switch typed := value.(type) {
	case float64:
		if typed > 0 {
			return int(typed)
		}
	case string:
		if parsed, err := strconv.Atoi(typed); err == nil && parsed > 0 {
			return parsed
		}
	}
	return fallback
}
