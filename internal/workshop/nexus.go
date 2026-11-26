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
	NexusEndpointName = "workshop-user-directory"
	NexusServiceName  = "user-directory"
	NexusGetUserOp    = "get-user"

	TeamANamespace = "team-a"
	TeamBNamespace = "team-b"
	TeamATaskQueue = "workshop-team-a"
	TeamBTaskQueue = "workshop-team-b"
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

func NewUserDirectoryService() (*nexus.Service, error) {
	operation := temporalnexus.NewWorkflowRunOperation(NexusGetUserOp, UserWorkflow,
		func(ctx context.Context, input GetUserRequest, options nexus.StartOperationOptions) (client.StartWorkflowOptions, error) {
			return client.StartWorkflowOptions{ID: "user-lookup-" + input.UserID}, nil
		})
	service := nexus.NewService(NexusServiceName)
	if err := service.Register(operation); err != nil {
		return nil, err
	}
	return service, nil
}

func PaymentWorkflow(ctx workflow.Context, request PaymentRequest) (PaymentReceipt, error) {
	if request.Amount <= 0 {
		return PaymentReceipt{}, fmt.Errorf("amount must be positive")
	}
	info := workflow.GetInfo(ctx)
	trace := []string{
		fmt.Sprintf("1. PAYMENT %s started in namespace %q (TeamA)", info.WorkflowExecution.ID, info.Namespace),
		fmt.Sprintf("2. TeamA calls %s/%s through endpoint %q — no custom RPC, no shared database", NexusServiceName, NexusGetUserOp, NexusEndpointName),
	}
	nexusClient := workflow.NewNexusClient(NexusEndpointName, NexusServiceName)
	future := nexusClient.ExecuteOperation(ctx, NexusGetUserOp, GetUserRequest{UserID: request.UserID},
		workflow.NexusOperationOptions{ScheduleToCloseTimeout: time.Minute})
	var profile UserProfile
	if err := future.Get(ctx, &profile); err != nil {
		return PaymentReceipt{}, fmt.Errorf("nexus call %s/%s: %w", NexusServiceName, NexusGetUserOp, err)
	}
	trace = append(trace,
		fmt.Sprintf("3. USER lookup for %s answered by namespace %q (TeamB): %s, plan %s", profile.UserID, profile.ServedBy, profile.Name, profile.Plan),
		fmt.Sprintf("4. TeamA charged %d credits and kept the receipt — durable execution spanned both namespaces", request.Amount),
	)
	return PaymentReceipt{
		PaymentID: info.WorkflowExecution.ID,
		UserID:    profile.UserID,
		Amount:    request.Amount,
		Status:    "charged",
		Trace:     trace,
	}, nil
}
