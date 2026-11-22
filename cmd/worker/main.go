package main

import (
	"log/slog"
	"os"

	"github.com/leowmjw/go-temporal-workshop-2026/internal/workshop"
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
	w.RegisterDynamicWorkflow(workshop.DynamicWorkflow, workflow.DynamicRegisterOptions{})
	w.RegisterDynamicActivity(workshop.DynamicActivity, activity.DynamicRegisterOptions{})

	slog.Info("worker started", "address", address, "task_queue", taskQueue)
	if err := w.Run(worker.InterruptCh()); err != nil {
		slog.Error("worker stopped", "error", err)
		os.Exit(1)
	}
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
