package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/leowmjw/go-temporal-workshop-2026/internal/workshop"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	address := env("TEMPORAL_ADDRESS", "127.0.0.1:7233")
	taskQueue := env("TEMPORAL_TASK_QUEUE", "workshop-main")
	c, err := client.Dial(client.Options{
		HostPort: address, Namespace: env("TEMPORAL_NAMESPACE", "default"),
	})
	if err != nil {
		slog.Error("dial Temporal", "error", err)
		os.Exit(1)
	}
	defer c.Close()

	w := worker.New(c, taskQueue, worker.Options{})
	w.RegisterWorkflow(workshop.StateWorkflow)
	w.RegisterActivity(workshop.StandaloneNotification)
	w.RegisterActivity(workshop.ForgeQuoteActivity)
	w.RegisterDynamicWorkflow(workshop.DynamicWorkflow, workflow.DynamicRegisterOptions{})
	w.RegisterDynamicActivity(workshop.DynamicActivity, activity.DynamicRegisterOptions{})

	workers := appendTeamWorkers([]worker.Worker{w}, address)
	for _, teamWorker := range workers {
		if err := teamWorker.Start(); err != nil {
			slog.Error("start worker", "error", err)
			os.Exit(1)
		}
	}

	slog.Info("worker started", "address", address, "task_queue", taskQueue, "workers", len(workers))
	<-worker.InterruptCh()
	for _, teamWorker := range workers {
		teamWorker.Stop()
	}
}

// appendTeamWorkers adds the TeamA (caller) and TeamB (Nexus handler) workers
// when the Nexus wiring exists. Without it the worker still serves the default
// namespace and reminds the operator to run `mise run setup-nexus`.
func appendTeamWorkers(workers []worker.Worker, address string) []worker.Worker {
	teamA, okA := joinNamespace(address, workshop.TeamANamespace)
	teamB, okB := joinNamespace(address, workshop.TeamBNamespace)
	if !okA || !okB {
		slog.Warn("Nexus team namespaces not wired; TeamA/TeamB workers disabled. Run `mise run setup-nexus` and restart the worker")
		return workers
	}

	wa := worker.New(teamA, workshop.TeamATaskQueue, worker.Options{})
	wa.RegisterWorkflow(workshop.PaymentWorkflow)
	workers = append(workers, wa)

	wb := worker.New(teamB, workshop.TeamBTaskQueue, worker.Options{})
	wb.RegisterWorkflow(workshop.UserWorkflow)
	service, err := workshop.NewUserDirectoryService()
	if err != nil {
		slog.Error("build Nexus service", "error", err)
		os.Exit(1)
	}
	wb.RegisterNexusService(service)
	workers = append(workers, wb)

	slog.Info("Nexus team workers started",
		"caller", workshop.TeamANamespace+"/"+workshop.TeamATaskQueue,
		"handler", workshop.TeamBNamespace+"/"+workshop.TeamBTaskQueue,
		"service", workshop.NexusServiceName)
	return workers
}

func joinNamespace(address, namespace string) (client.Client, bool) {
	teamClient, err := client.Dial(client.Options{HostPort: address, Namespace: namespace})
	if err != nil {
		slog.Warn("dial team namespace", "namespace", namespace, "error", err)
		return nil, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := teamClient.WorkflowService().DescribeNamespace(ctx, &workflowservice.DescribeNamespaceRequest{Namespace: namespace}); err != nil {
		teamClient.Close()
		return nil, false
	}
	return teamClient, true
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
