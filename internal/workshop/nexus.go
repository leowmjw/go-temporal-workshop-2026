package workshop

import (
	"context"
	"fmt"
	"time"

	"github.com/nexus-rpc/sdk-go/nexus"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporalnexus"
	"go.temporal.io/sdk/workflow"
)

const (
	NexusEndpointName   = "workshop-user-directory"
	NexusServiceName    = "user-directory"
	NexusGetUserOp      = "get-user"
	NexusReviewCreditOp = "review-credit"

	TeamANamespace = "team-a"
	TeamBNamespace = "team-b"
	TeamATaskQueue = "workshop-team-a"
	TeamBTaskQueue = "workshop-team-b"

	// CreditLimit is TeamB's workshop lending rule: amounts above it are declined.
	CreditLimit = 250
)

type GetUserRequest struct {
	UserID string `json:"userId"`
}

type UserProfile struct {
	UserID   string `json:"userId"`
	Name     string `json:"name"`
	Plan     string `json:"plan"`
	ServedBy string `json:"servedBy"`
}

type PaymentRequest struct {
	UserID string `json:"userId"`
	Amount int    `json:"amount"`
}

type PaymentReceipt struct {
	PaymentID string   `json:"paymentId"`
	UserID    string   `json:"userId"`
	Amount    int      `json:"amount"`
	Status    string   `json:"status"`
	Trace     []string `json:"trace"`
}

type CreditReviewRequest struct {
	UserID string `json:"userId"`
	Amount int    `json:"amount"`
}

type CreditReviewResult struct {
	UserID     string   `json:"userId"`
	Amount     int      `json:"amount"`
	Decision   string   `json:"decision"`
	Reason     string   `json:"reason"`
	Stages     []string `json:"stages"`
	ReviewedBy string   `json:"reviewedBy"`
}

var creditReviewStages = []string{
	"verifying identity",
	"checking exposure",
	"scoring payment history",
	"countersigning decision",
}

func UserWorkflow(ctx workflow.Context, request GetUserRequest) (UserProfile, error) {
	info := workflow.GetInfo(ctx)
	if err := workflow.Sleep(ctx, 300*time.Millisecond); err != nil {
		return UserProfile{}, err
	}
	return UserProfile{
		UserID:   request.UserID,
		Name:     "User " + request.UserID,
		Plan:     "workshop",
		ServedBy: info.Namespace,
	}, nil
}

// CreditReviewWorkflow is a substantive Nexus handler: a long-running,
// multi-stage review with a queryable progress stage and a durable verdict.
// TeamA attaches to it through Nexus and awaits the promise.
func CreditReviewWorkflow(ctx workflow.Context, request CreditReviewRequest) (CreditReviewResult, error) {
	info := workflow.GetInfo(ctx)
	stage := "queued"
	if err := workflow.SetQueryHandler(ctx, "Stage", func() (string, error) {
		return stage, nil
	}); err != nil {
		return CreditReviewResult{}, err
	}
	done := make([]string, 0, len(creditReviewStages))
	for _, next := range creditReviewStages {
		stage = next
		if err := workflow.Sleep(ctx, 1200*time.Millisecond); err != nil {
			return CreditReviewResult{}, err
		}
		done = append(done, next)
	}
	decision, reason := "approved", fmt.Sprintf("within workshop credit limit of %d", CreditLimit)
	if request.Amount > CreditLimit {
		decision, reason = "declined", fmt.Sprintf("exceeds workshop credit limit of %d", CreditLimit)
	}
	return CreditReviewResult{
		UserID:     request.UserID,
		Amount:     request.Amount,
		Decision:   decision,
		Reason:     reason,
		Stages:     done,
		ReviewedBy: info.Namespace,
	}, nil
}

func NewUserDirectoryService() (*nexus.Service, error) {
	getUser := temporalnexus.NewWorkflowRunOperation(NexusGetUserOp, UserWorkflow,
		func(ctx context.Context, input GetUserRequest, options nexus.StartOperationOptions) (client.StartWorkflowOptions, error) {
			return client.StartWorkflowOptions{ID: "user-lookup-" + input.UserID}, nil
		})
	reviewCredit := temporalnexus.NewWorkflowRunOperation(NexusReviewCreditOp, CreditReviewWorkflow,
		func(ctx context.Context, input CreditReviewRequest, options nexus.StartOperationOptions) (client.StartWorkflowOptions, error) {
			// Deterministic ID makes the operation idempotent: a retried or repeated
			// call for the same user+amount attaches to the same review run.
			return client.StartWorkflowOptions{ID: fmt.Sprintf("credit-review-%s-%d", input.UserID, input.Amount)}, nil
		})
	service := nexus.NewService(NexusServiceName)
	if err := service.Register(getUser, reviewCredit); err != nil {
		return nil, err
	}
	return service, nil
}

func PaymentWorkflow(ctx workflow.Context, request PaymentRequest) (PaymentReceipt, error) {
	if request.Amount <= 0 {
		return PaymentReceipt{}, fmt.Errorf("amount must be positive")
	}
	info := workflow.GetInfo(ctx)
	receipt := PaymentReceipt{
		PaymentID: info.WorkflowExecution.ID,
		UserID:    request.UserID,
		Amount:    request.Amount,
		Trace: []string{
			fmt.Sprintf("1. PAYMENT %s started in namespace %q (TeamA)", info.WorkflowExecution.ID, info.Namespace),
			fmt.Sprintf("2. quick typed lookup: %s/%s through endpoint %q", NexusServiceName, NexusGetUserOp, NexusEndpointName),
		},
	}
	nexusClient := workflow.NewNexusClient(NexusEndpointName, NexusServiceName)

	profileFuture := nexusClient.ExecuteOperation(ctx, NexusGetUserOp, GetUserRequest{UserID: request.UserID},
		workflow.NexusOperationOptions{ScheduleToCloseTimeout: time.Minute})
	var profile UserProfile
	if err := profileFuture.Get(ctx, &profile); err != nil {
		return PaymentReceipt{}, fmt.Errorf("nexus call %s/%s: %w", NexusServiceName, NexusGetUserOp, err)
	}
	receipt.Trace = append(receipt.Trace,
		fmt.Sprintf("3. USER lookup for %s answered by namespace %q (TeamB): %s, plan %s", profile.UserID, profile.ServedBy, profile.Name, profile.Plan),
		fmt.Sprintf("4. durable promise: %s/%s — TeamB runs a real CreditReviewWorkflow; TeamA only awaits the verdict", NexusServiceName, NexusReviewCreditOp),
	)

	reviewFuture := nexusClient.ExecuteOperation(ctx, NexusReviewCreditOp, CreditReviewRequest{UserID: request.UserID, Amount: request.Amount},
		workflow.NexusOperationOptions{ScheduleToCloseTimeout: 5 * time.Minute})
	var review CreditReviewResult
	if err := reviewFuture.Get(ctx, &review); err != nil {
		return PaymentReceipt{}, fmt.Errorf("nexus call %s/%s: %w", NexusServiceName, NexusReviewCreditOp, err)
	}
	receipt.Trace = append(receipt.Trace,
		fmt.Sprintf("5. review %s by namespace %q after %d stages (%s)", review.Decision, review.ReviewedBy, len(review.Stages), review.Reason),
	)

	if review.Decision != "approved" {
		receipt.Status = "declined"
		receipt.Trace = append(receipt.Trace, "6. TeamA kept the decline as a durable receipt — no charge, no callback to TeamB")
		return receipt, nil
	}
	receipt.Status = "charged"
	receipt.Trace = append(receipt.Trace,
		fmt.Sprintf("6. TeamA charged %d credits — execution spanned both namespaces and TeamB's long review without a single custom RPC", request.Amount),
	)
	return receipt, nil
}
