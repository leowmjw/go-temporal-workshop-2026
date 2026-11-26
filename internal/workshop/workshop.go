package workshop

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/workflow"
)

type State struct {
	Counter int      `json:"counter"`
	Notes   []string `json:"notes"`
}

type UpdateRequest struct {
	Delta int    `json:"delta"`
	Note  string `json:"note,omitempty"`
}

func StateWorkflow(ctx workflow.Context, initial int) (State, error) {
	state := State{Counter: initial, Notes: []string{}}
	if err := workflow.SetUpdateHandler(ctx, "Apply", func(ctx workflow.Context, request UpdateRequest) (State, error) {
		if request.Delta == 0 {
			return state, fmt.Errorf("delta must be non-zero")
		}
		state.Counter += request.Delta
		note := request.Note
		if note == "" {
			note = fmt.Sprintf("applied delta %d", request.Delta)
		}
		state.Notes = append(state.Notes, note)
		if len(state.Notes) > 20 {
			state.Notes = state.Notes[len(state.Notes)-20:]
		}
		return state, nil
	}); err != nil {
		return State{}, err
	}
	if err := workflow.SetQueryHandler(ctx, "Snapshot", func() (State, error) {
		return state, nil
	}); err != nil {
		return State{}, err
	}
	_ = workflow.Sleep(ctx, 10*time.Minute)
	return state, nil
}

func StandaloneNotification(ctx context.Context, audience string) (string, error) {
	time.Sleep(50 * time.Millisecond)
	return fmt.Sprintf("durable standalone activity delivered to %s", audience), nil
}

type Quote struct {
	ID     int    `json:"id"`
	Text   string `json:"text"`
	Author string `json:"author"`
}

var Quotes = []Quote{
	{1, "Simplicity is prerequisite for reliability.", "Edsger W. Dijkstra"},
	{2, "Make it work, make it right, make it fast.", "Kent Beck"},
	{3, "Premature optimization is the root of all evil.", "Donald Knuth"},
	{4, "Programs must be written for people to read, and only incidentally for machines to execute.", "Harold Abelson"},
	{5, "The best error message is the one that never shows up.", "Thomas Fuchs"},
	{6, "There are only two hard things in computer science: cache invalidation and naming things.", "Phil Karlton"},
	{7, "A ship in harbor is safe, but that is not what ships are built for.", "John A. Shedd"},
	{8, "The most dangerous phrase in the language is, 'We've always done it this way.'", "Grace Hopper"},
	{9, "Any fool can write code that a computer can understand. Good programmers write code that humans can understand.", "Martin Fowler"},
	{10, "Talk is cheap. Show me the code.", "Linus Torvalds"},
}

var ForgeStages = []string{
	"selecting scraps",
	"weighing words",
	"smoothing syntax",
	"tempering cadence",
	"polishing pauses",
	"balancing brevity",
	"checking attribution",
	"sealing the quote",
}

const ForgeSteps = 16

// forgeStepDelay paces the simulated work. It is long enough that a running
// forge outlives a heartbeat throttle window, so pause/cancel are observable.
// Stored atomically because tests adjust it while running in parallel.
var forgeStepDelay atomic.Int64

func init() { forgeStepDelay.Store(int64(600 * time.Millisecond)) }

func currentForgeStepDelay() time.Duration { return time.Duration(forgeStepDelay.Load()) }

type ForgeProgress struct {
	QuoteID int    `json:"quoteId"`
	Step    int    `json:"step"`
	Steps   int    `json:"steps"`
	Stage   string `json:"stage"`
}

// ForgeQuoteActivity deliberately runs long enough to be paused, canceled, and
// inspected. It heartbeats every step so a restarted or unpaused attempt resumes
// where it left off instead of starting over.
func ForgeQuoteActivity(ctx context.Context, quoteID int) (Quote, error) {
	quote, ok := QuoteByID(quoteID)
	if !ok {
		return Quote{}, fmt.Errorf("no quote with id %d", quoteID)
	}

	start := 0
	var previous ForgeProgress
	if activity.HasHeartbeatDetails(ctx) {
		if err := activity.GetHeartbeatDetails(ctx, &previous); err == nil {
			start = previous.Step
		}
	}

	for step := start; step < ForgeSteps; step++ {
		select {
		case <-ctx.Done():
			return Quote{}, ctx.Err()
		case <-time.After(currentForgeStepDelay()):
		}

		stage := ForgeStages[step*len(ForgeStages)/ForgeSteps]
		activity.RecordHeartbeat(ctx, ForgeProgress{
			QuoteID: quoteID,
			Step:    step + 1,
			Steps:   ForgeSteps,
			Stage:   stage,
		})
		if ctx.Err() != nil {
			return Quote{}, ctx.Err()
		}
	}

	return quote, nil
}

func QuoteByID(id int) (Quote, bool) {
	for _, quote := range Quotes {
		if quote.ID == id {
			return quote, true
		}
	}
	return Quote{}, false
}

func DynamicWorkflow(ctx workflow.Context, args converter.EncodedValues) (string, error) {
	info := workflow.GetInfo(ctx)
	var payload string
	if err := args.Get(&payload); err != nil {
		return "", err
	}
	return fmt.Sprintf(`{"dynamicWorkflow":"%s","args":%q}`, info.WorkflowType.Name, payload), nil
}

func DynamicActivity(ctx context.Context, args converter.EncodedValues) (string, error) {
	info := activity.GetInfo(ctx)
	var payload string
	if err := args.Get(&payload); err != nil {
		return "", err
	}
	return fmt.Sprintf(`{"dynamicActivity":"%s","args":%q}`, info.ActivityType.Name, payload), nil
}
