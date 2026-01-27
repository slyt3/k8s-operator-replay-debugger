//go:build ignore

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/operator-replay-debugger/pkg/recorder"
	"github.com/operator-replay-debugger/pkg/storage"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	maxReconcileLoops = 100
)

// This example shows how to integrate the recording client
// into a Kubernetes operator reconciliation loop.

func main() {
	fmt.Println("Kubernetes Operator Recording Example")
	fmt.Println("Note: This requires a working Kubernetes cluster")
	fmt.Println()

	config, err := rest.InClusterConfig()
	if err != nil {
		fmt.Printf("Not running in cluster, trying kubeconfig: %v\n", err)
		return
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Printf("Failed to create clientset: %v\n", err)
		return
	}

	db, err := storage.NewDatabase("operator_recordings.db", 1000000)
	if err != nil {
		fmt.Printf("Failed to create database: %v\n", err)
		return
	}
	defer func() {
		closeErr := db.Close()
		if closeErr != nil {
			fmt.Printf("Warning: database close failed: %v\n", closeErr)
		}
	}()

	sessionID := fmt.Sprintf("operator-run-%d", time.Now().Unix())

	recordingClient, err := recorder.NewRecordingClient(recorder.Config{
		Client:      clientset,
		Database:    db,
		SessionID:   sessionID,
		MaxSequence: 100000,
	})
	if err != nil {
		fmt.Printf("Failed to create recording client: %v\n", err)
		return
	}

	fmt.Printf("Recording session: %s\n", sessionID)
	fmt.Println("Starting reconciliation loop...")

	err = runReconciliationLoop(recordingClient)
	if err != nil {
		fmt.Printf("Reconciliation failed: %v\n", err)
		return
	}

	fmt.Printf("\nRecording complete: %d operations\n",
		recordingClient.GetSequenceNumber())
	fmt.Printf("Database: operator_recordings.db\n")
	fmt.Printf("Session: %s\n", sessionID)
	fmt.Println("\nReplay with:")
	fmt.Printf("  ./replay-cli replay %s -d operator_recordings.db\n", sessionID)
}

// runReconciliationLoop simulates an operator reconciliation loop.
func runReconciliationLoop(client *recorder.RecordingClient) error {
	ctx := context.Background()

	loopCount := 0
	for loopCount < maxReconcileLoops {
		err := reconcile(ctx, client)
		if err != nil {
			return fmt.Errorf("reconcile failed at iteration %d: %w",
				loopCount, err)
		}

		time.Sleep(5 * time.Second)
		loopCount = loopCount + 1
	}

	return nil
}

// reconcile performs one reconciliation cycle.
func reconcile(ctx context.Context, client *recorder.RecordingClient) error {
	_, err := client.RecordGet(
		ctx,
		"Pod",
		"default",
		"example-pod",
		metav1.GetOptions{},
	)
	if err != nil {
		fmt.Printf("  Warning: Get Pod failed: %v\n", err)
	}

	_, err = client.RecordGet(
		ctx,
		"Service",
		"default",
		"example-service",
		metav1.GetOptions{},
	)
	if err != nil {
		fmt.Printf("  Warning: Get Service failed: %v\n", err)
	}

	fmt.Printf("  Reconciled at seq %d\n", client.GetSequenceNumber())
	return nil
}
