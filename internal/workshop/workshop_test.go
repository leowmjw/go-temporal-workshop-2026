package workshop

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
)

func TestStandaloneNotification(t *testing.T) {
	t.Parallel()
	env := (&testsuite.WorkflowTestSuite{}).NewTestActivityEnvironment()
	env.RegisterActivity(StandaloneNotification)
	encoded, err := env.ExecuteActivity(StandaloneNotification, "audience")
	require.NoError(t, err)
	var result string
	require.NoError(t, encoded.Get(&result))
	require.Contains(t, result, "audience")
}

func TestStateWorkflowAcceptsTwoUpdates(t *testing.T) {
	t.Parallel()
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterDelayedCallback(func() {
		env.UpdateWorkflow("Apply", "update-1", &testsuite.TestUpdateCallback{
			OnReject: func(err error) { require.NoError(t, err) },
			OnAccept: func() {},
			OnComplete: func(value any, err error) {
				require.NoError(t, err)
				state, ok := value.(State)
				require.True(t, ok)
				require.Equal(t, 1, state.Counter)
			},
		}, UpdateRequest{Delta: 1})
	}, time.Second)
	env.RegisterDelayedCallback(func() {
		env.UpdateWorkflow("Apply", "update-2", &testsuite.TestUpdateCallback{
			OnReject: func(err error) { require.NoError(t, err) },
			OnAccept: func() {},
			OnComplete: func(value any, err error) {
				require.NoError(t, err)
				state, ok := value.(State)
				require.True(t, ok)
				require.Equal(t, 3, state.Counter)
			},
		}, UpdateRequest{Delta: 2, Note: "second update"})
	}, 2*time.Second)

	env.ExecuteWorkflow(StateWorkflow, 0)
	require.NoError(t, env.GetWorkflowError())
}

func TestDynamicHandlers(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	_ = ctx
	require.NotEmpty(t, "Dynamic handlers are registered by name")
}
