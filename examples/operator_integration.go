//go:build ignore

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/slyt3/kubestep/pkg/recorder"
	"github.com/slyt3/kubestep/pkg/storage"
	corev1 "k8s.io/api/core/v1"
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
	fmt.Println("KubeStep Recording Example")
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
		ActorID:     "example-operator/controller",
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
	fmt.Printf("  ./kubestep replay %s -d operator_recordings.db\n", sessionID)
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

	err = recordConfigMapLifecycle(ctx, client)
	if err != nil {
		fmt.Printf("  Warning: ConfigMap lifecycle failed: %v\n", err)
	}

	err = recordSecretLifecycle(ctx, client)
	if err != nil {
		fmt.Printf("  Warning: Secret lifecycle failed: %v\n", err)
	}

	fmt.Printf("  Reconciled at seq %d\n", client.GetSequenceNumber())
	return nil
}

func recordConfigMapLifecycle(ctx context.Context, client *recorder.RecordingClient) error {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-configmap",
			Namespace: "default",
		},
		Data: map[string]string{
			"status": "created",
		},
	}

	created, err := client.RecordCreate(ctx, "ConfigMap", "default", configMap, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create configmap failed: %w", err)
	}

	createdConfigMap, ok := created.(*corev1.ConfigMap)
	if !ok {
		return fmt.Errorf("create configmap returned unexpected type: %T", created)
	}

	if createdConfigMap.Data == nil {
		createdConfigMap.Data = map[string]string{}
	}
	createdConfigMap.Data["status"] = "updated"

	_, err = client.RecordUpdate(ctx, "ConfigMap", "default", createdConfigMap, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("update configmap failed: %w", err)
	}

	err = client.RecordDelete(ctx, "ConfigMap", "default", createdConfigMap.Name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("delete configmap failed: %w", err)
	}

	return nil
}

func recordSecretLifecycle(ctx context.Context, client *recorder.RecordingClient) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"token": []byte("initial"),
		},
	}

	created, err := client.RecordCreate(ctx, "Secret", "default", secret, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create secret failed: %w", err)
	}

	createdSecret, ok := created.(*corev1.Secret)
	if !ok {
		return fmt.Errorf("create secret returned unexpected type: %T", created)
	}

	if createdSecret.Data == nil {
		createdSecret.Data = map[string][]byte{}
	}
	createdSecret.Data["token"] = []byte("updated")

	_, err = client.RecordUpdate(ctx, "Secret", "default", createdSecret, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("update secret failed: %w", err)
	}

	err = client.RecordDelete(ctx, "Secret", "default", createdSecret.Name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("delete secret failed: %w", err)
	}

	return nil
}
