package workshop

import (
	"context"
	"fmt"
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
