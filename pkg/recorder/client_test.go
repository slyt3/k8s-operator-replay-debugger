package recorder

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/slyt3/kubestep/pkg/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

const (
	testSessionID = "recorder-session-001"
)

func newTestRecorder(t *testing.T, client kubernetes.Interface) (*RecordingClient, *storage.Database) {
	t.Helper()

	require.NotNil(t, client)

	dbPath := filepath.Join(t.TempDir(), "recordings.db")
	db, err := storage.NewDatabase(dbPath, 1000)
	require.NoError(t, err)

	rec, err := NewRecordingClient(Config{
		Client:      client,
		Database:    db,
		SessionID:   testSessionID,
		MaxSequence: 100,
	})
	require.NoError(t, err)
	require.NotNil(t, rec)

	t.Cleanup(func() {
		assert.NoError(t, db.Close())
	})

	return rec, db
}

func TestRecordGetConfigMapAndSecret(t *testing.T) {
	ctx := context.Background()

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "config-get",
			Namespace: "default",
		},
		Data: map[string]string{
			"mode": "test",
		},
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "secret-get",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"token": []byte("abc"),
		},
	}

	client := fake.NewSimpleClientset(configMap, secret)
	rec, db := newTestRecorder(t, client)

	obj, err := rec.RecordGet(ctx, "ConfigMap", "default", "config-get", metav1.GetOptions{})
	require.NoError(t, err)
	require.NotNil(t, obj)

	obj, err = rec.RecordGet(ctx, "Secret", "default", "secret-get", metav1.GetOptions{})
	require.NoError(t, err)
	require.NotNil(t, obj)

	ops, err := db.QueryOperations(testSessionID)
	require.NoError(t, err)
	require.Len(t, ops, 2)

	assert.Equal(t, storage.OperationGet, ops[0].OperationType)
	assert.Equal(t, "ConfigMap", ops[0].ResourceKind)
	assert.Equal(t, "config-get", ops[0].Name)
	assert.Equal(t, "", ops[0].Error)

	assert.Equal(t, storage.OperationGet, ops[1].OperationType)
	assert.Equal(t, "Secret", ops[1].ResourceKind)
	assert.Equal(t, "secret-get", ops[1].Name)
	assert.Equal(t, "", ops[1].Error)
}

func TestRecordCreateConfigMapAndSecret(t *testing.T) {
	ctx := context.Background()

	client := fake.NewSimpleClientset()
	rec, db := newTestRecorder(t, client)

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "config-create",
			Namespace: "default",
		},
		Data: map[string]string{
			"mode": "create",
		},
	}
	created, err := rec.RecordCreate(ctx, "ConfigMap", "default", configMap, metav1.CreateOptions{})
	require.NoError(t, err)
	require.NotNil(t, created)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "secret-create",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"token": []byte("abc"),
		},
	}
	created, err = rec.RecordCreate(ctx, "Secret", "default", secret, metav1.CreateOptions{})
	require.NoError(t, err)
	require.NotNil(t, created)

	ops, err := db.QueryOperations(testSessionID)
	require.NoError(t, err)
	require.Len(t, ops, 2)

	assert.Equal(t, storage.OperationCreate, ops[0].OperationType)
	assert.Equal(t, "ConfigMap", ops[0].ResourceKind)
	assert.Equal(t, "config-create", ops[0].Name)
	assert.Equal(t, "", ops[0].Error)

	assert.Equal(t, storage.OperationCreate, ops[1].OperationType)
	assert.Equal(t, "Secret", ops[1].ResourceKind)
	assert.Equal(t, "secret-create", ops[1].Name)
	assert.Equal(t, "", ops[1].Error)
}

func TestRecordUpdateConfigMapAndSecret(t *testing.T) {
	ctx := context.Background()

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "config-update",
			Namespace: "default",
		},
		Data: map[string]string{
			"mode": "before",
		},
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "secret-update",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"token": []byte("before"),
		},
	}

	client := fake.NewSimpleClientset(configMap, secret)
	rec, db := newTestRecorder(t, client)

	configMapUpdate := configMap.DeepCopy()
	configMapUpdate.Data["mode"] = "after"

	updated, err := rec.RecordUpdate(ctx, "ConfigMap", "default", configMapUpdate, metav1.UpdateOptions{})
	require.NoError(t, err)
	require.NotNil(t, updated)

	secretUpdate := secret.DeepCopy()
	secretUpdate.Data["token"] = []byte("after")

	updated, err = rec.RecordUpdate(ctx, "Secret", "default", secretUpdate, metav1.UpdateOptions{})
	require.NoError(t, err)
	require.NotNil(t, updated)

	ops, err := db.QueryOperations(testSessionID)
	require.NoError(t, err)
	require.Len(t, ops, 2)

	assert.Equal(t, storage.OperationUpdate, ops[0].OperationType)
	assert.Equal(t, "ConfigMap", ops[0].ResourceKind)
	assert.Equal(t, "config-update", ops[0].Name)
	assert.Equal(t, "", ops[0].Error)

	assert.Equal(t, storage.OperationUpdate, ops[1].OperationType)
	assert.Equal(t, "Secret", ops[1].ResourceKind)
	assert.Equal(t, "secret-update", ops[1].Name)
	assert.Equal(t, "", ops[1].Error)
}

func TestRecordDeleteConfigMapAndSecret(t *testing.T) {
	ctx := context.Background()

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "config-delete",
			Namespace: "default",
		},
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "secret-delete",
			Namespace: "default",
		},
	}

	client := fake.NewSimpleClientset(configMap, secret)
	rec, db := newTestRecorder(t, client)

	err := rec.RecordDelete(ctx, "ConfigMap", "default", "config-delete", metav1.DeleteOptions{})
	require.NoError(t, err)

	err = rec.RecordDelete(ctx, "Secret", "default", "secret-delete", metav1.DeleteOptions{})
	require.NoError(t, err)

	ops, err := db.QueryOperations(testSessionID)
	require.NoError(t, err)
	require.Len(t, ops, 2)

	assert.Equal(t, storage.OperationDelete, ops[0].OperationType)
	assert.Equal(t, "ConfigMap", ops[0].ResourceKind)
	assert.Equal(t, "config-delete", ops[0].Name)
	assert.Equal(t, "", ops[0].Error)

	assert.Equal(t, storage.OperationDelete, ops[1].OperationType)
	assert.Equal(t, "Secret", ops[1].ResourceKind)
	assert.Equal(t, "secret-delete", ops[1].Name)
	assert.Equal(t, "", ops[1].Error)
}
