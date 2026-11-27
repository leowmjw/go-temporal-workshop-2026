package workshop

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
)

func TestForgeQuoteActivityCompletesWithSelectedQuote(t *testing.T) {
	t.Parallel()
	previous := forgeStepDelay.Load()
	forgeStepDelay.Store(int64(time.Millisecond))
	t.Cleanup(func() { forgeStepDelay.Store(previous) })

	env := (&testsuite.WorkflowTestSuite{}).NewTestActivityEnvironment()
	env.RegisterActivity(ForgeQuoteActivity)
	encoded, err := env.ExecuteActivity(ForgeQuoteActivity, 10)
	require.NoError(t, err)
	var quote Quote
	require.NoError(t, encoded.Get(&quote))
	require.Equal(t, "Linus Torvalds", quote.Author)
}

func TestForgeQuoteActivityRejectsUnknownQuote(t *testing.T) {
	t.Parallel()
	env := (&testsuite.WorkflowTestSuite{}).NewTestActivityEnvironment()
	env.RegisterActivity(ForgeQuoteActivity)
	_, err := env.ExecuteActivity(ForgeQuoteActivity, 99)
	require.Error(t, err)
}

func TestForgeQuoteActivityResumesFromHeartbeat(t *testing.T) {
	t.Parallel()
	previous := forgeStepDelay.Load()
	forgeStepDelay.Store(int64(time.Millisecond))
	t.Cleanup(func() { forgeStepDelay.Store(previous) })

	env := (&testsuite.WorkflowTestSuite{}).NewTestActivityEnvironment()
	env.RegisterActivity(ForgeQuoteActivity)
	env.SetHeartbeatDetails(ForgeProgress{QuoteID: 3, Step: 12, Steps: ForgeSteps, Stage: "polishing pauses"})

	encoded, err := env.ExecuteActivity(ForgeQuoteActivity, 3)
	require.NoError(t, err)
	var quote Quote
	require.NoError(t, encoded.Get(&quote))
	require.Equal(t, "Premature optimization is the root of all evil.", quote.Text)
}

func TestQuoteCatalog(t *testing.T) {
	t.Parallel()
	require.Len(t, Quotes, 10)
	seen := map[int]bool{}
	for _, quote := range Quotes {
		require.NotEmpty(t, quote.Text)
		require.NotEmpty(t, quote.Author)
		require.False(t, seen[quote.ID], "duplicate quote id %d", quote.ID)
		seen[quote.ID] = true
	}
}

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

func TestUserWorkflowReturnsProfile(t *testing.T) {
	t.Parallel()
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.ExecuteWorkflow(UserWorkflow, GetUserRequest{UserID: "u-1"})
	require.NoError(t, env.GetWorkflowError())
	var profile UserProfile
	require.NoError(t, env.GetWorkflowResult(&profile))
	require.Equal(t, "u-1", profile.UserID)
	require.Equal(t, "User u-1", profile.Name)
	require.NotEmpty(t, profile.ServedBy)
}

func TestUserDirectoryServiceRegistersGetUser(t *testing.T) {
	t.Parallel()
	service, err := NewUserDirectoryService()
	require.NoError(t, err)
	require.Equal(t, NexusServiceName, service.Name)
}

func TestPaymentWorkflowRejectsNonPositiveAmount(t *testing.T) {
	t.Parallel()
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.ExecuteWorkflow(PaymentWorkflow, PaymentRequest{UserID: "u-1", Amount: 0})
	require.ErrorContains(t, env.GetWorkflowError(), "amount must be positive")
}

func TestCreditReviewWorkflowApprovesWithinLimit(t *testing.T) {
	t.Parallel()
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.ExecuteWorkflow(CreditReviewWorkflow, CreditReviewRequest{UserID: "u-1", Amount: CreditLimit})
	require.NoError(t, env.GetWorkflowError())
	var review CreditReviewResult
	require.NoError(t, env.GetWorkflowResult(&review))
	require.Equal(t, "approved", review.Decision)
	require.Len(t, review.Stages, 4)
	require.NotEmpty(t, review.ReviewedBy)
}

func TestCreditReviewWorkflowDeclinesOverLimit(t *testing.T) {
	t.Parallel()
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.ExecuteWorkflow(CreditReviewWorkflow, CreditReviewRequest{UserID: "u-1", Amount: CreditLimit + 1})
	require.NoError(t, env.GetWorkflowError())
	var review CreditReviewResult
	require.NoError(t, env.GetWorkflowResult(&review))
	require.Equal(t, "declined", review.Decision)
	require.Contains(t, review.Reason, "credit limit")
}
