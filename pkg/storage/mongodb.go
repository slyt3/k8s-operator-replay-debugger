package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/operator-replay-debugger/internal/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoStore implements OperationStore using MongoDB.
type MongoStore struct {
	client         *mongo.Client
	database       *mongo.Database
	collection     *mongo.Collection
	spanCollection *mongo.Collection
	maxOperations  int
	ctx            context.Context
}

// MongoOperation represents an operation document in MongoDB.
type MongoOperation struct {
	ID              string    `bson:"_id,omitempty"`
	SessionID       string    `bson:"session_id"`
	SequenceNumber  int64     `bson:"sequence_number"`
	Timestamp       time.Time `bson:"timestamp"`
	OperationType   string    `bson:"operation_type"`
	ResourceKind    string    `bson:"resource_kind"`
	Namespace       string    `bson:"namespace,omitempty"`
	Name            string    `bson:"name,omitempty"`
	ResourceData    string    `bson:"resource_data,omitempty"`
	Error           string    `bson:"error,omitempty"`
	DurationMs      int64     `bson:"duration_ms"`
	ActorID         string    `bson:"actor_id,omitempty"`
	UID             string    `bson:"uid,omitempty"`
	ResourceVersion string    `bson:"resource_version,omitempty"`
	Generation      int64     `bson:"generation,omitempty"`
	Verb            string    `bson:"verb,omitempty"`
}

// MongoReconcileSpan represents a reconcile span document in MongoDB.
type MongoReconcileSpan struct {
	ID                     string     `bson:"_id,omitempty"`
	SessionID              string     `bson:"session_id"`
	ActorID                string     `bson:"actor_id"`
	StartTime              time.Time  `bson:"start_ts"`
	EndTime                *time.Time `bson:"end_ts,omitempty"`
	DurationMs             *int64     `bson:"duration_ms,omitempty"`
	Kind                   string     `bson:"kind"`
	Namespace              string     `bson:"namespace,omitempty"`
	Name                   string     `bson:"name,omitempty"`
	TriggerUID             string     `bson:"trigger_uid,omitempty"`
	TriggerResourceVersion string     `bson:"trigger_resource_version,omitempty"`
	TriggerReason          string     `bson:"trigger_reason,omitempty"`
	Error                  string     `bson:"error,omitempty"`
}

// NewMongoStore creates a new MongoDB-based operation store.
func NewMongoStore(cfg StorageConfig) (*MongoStore, error) {
	ctx := cfg.Context
	if ctx == nil {
		ctx = context.Background()
	}

	clientOpts := options.Client().ApplyURI(cfg.ConnectionURI)
	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		closeErr := client.Disconnect(ctx)
		if closeErr != nil {
			return nil, fmt.Errorf("ping failed: %w, disconnect failed: %v",
				err, closeErr)
		}
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	database := client.Database(cfg.DatabaseName)
	collection := database.Collection(cfg.CollectionName)
	spanCollection := database.Collection("reconcile_spans")

	store := &MongoStore{
		client:         client,
		database:       database,
		collection:     collection,
		spanCollection: spanCollection,
		maxOperations:  cfg.MaxOperations,
		ctx:            ctx,
	}

	err = store.createIndexes()
	if err != nil {
		closeErr := client.Disconnect(ctx)
		if closeErr != nil {
			return nil, fmt.Errorf("index creation failed: %w, disconnect failed: %v",
				err, closeErr)
		}
		return nil, err
	}

	return store, nil
}

// InsertOperation inserts a single operation record.
func (m *MongoStore) InsertOperation(op *Operation) error {
	err := assert.AssertNotNil(op, "operation")
	if err != nil {
		return err
	}

	err = ValidateOperation(op)
	if err != nil {
		return fmt.Errorf("invalid operation: %w", err)
	}

	mongoOp := MongoOperation{
		SessionID:       op.SessionID,
		SequenceNumber:  op.SequenceNumber,
		Timestamp:       op.Timestamp,
		OperationType:   string(op.OperationType),
		ResourceKind:    op.ResourceKind,
		Namespace:       op.Namespace,
		Name:            op.Name,
		ResourceData:    op.ResourceData,
		Error:           op.Error,
		DurationMs:      op.DurationMs,
		ActorID:         op.ActorID,
		UID:             op.UID,
		ResourceVersion: op.ResourceVersion,
		Generation:      op.Generation,
		Verb:            op.Verb,
	}

	_, err = m.collection.InsertOne(m.ctx, mongoOp)
	if err != nil {
		return fmt.Errorf("failed to insert operation: %w", err)
	}

	return nil
}

// QueryOperations retrieves all operations for a session.
func (m *MongoStore) QueryOperations(sessionID string) ([]Operation, error) {
	err := assert.AssertStringNotEmpty(sessionID, "session ID")
	if err != nil {
		return nil, err
	}

	filter := bson.M{"session_id": sessionID}
	opts := options.Find().
		SetSort(bson.M{"sequence_number": 1}).
		SetLimit(int64(maxQueryResults))

	cursor, err := m.collection.Find(m.ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer func() {
		closeErr := cursor.Close(m.ctx)
		if closeErr != nil {
			fmt.Printf("Warning: failed to close cursor: %v\n", closeErr)
		}
	}()

	return m.scanOperations(cursor)
}

// InsertReconcileSpan inserts a reconcile span record.
func (m *MongoStore) InsertReconcileSpan(span *ReconcileSpan) error {
	err := assert.AssertNotNil(span, "reconcile span")
	if err != nil {
		return err
	}

	err = ValidateReconcileSpan(span)
	if err != nil {
		return fmt.Errorf("invalid span: %w", err)
	}

	mongoSpan := MongoReconcileSpan{
		ID:                     span.ID,
		SessionID:              span.SessionID,
		ActorID:                span.ActorID,
		StartTime:              span.StartTime,
		Kind:                   span.Kind,
		Namespace:              span.Namespace,
		Name:                   span.Name,
		TriggerUID:             span.TriggerUID,
		TriggerResourceVersion: span.TriggerResourceVersion,
		TriggerReason:          span.TriggerReason,
		Error:                  span.Error,
	}

	if !span.EndTime.IsZero() {
		endTime := span.EndTime
		mongoSpan.EndTime = &endTime
	}
	if span.DurationMs > 0 {
		duration := span.DurationMs
		mongoSpan.DurationMs = &duration
	}

	_, err = m.spanCollection.InsertOne(m.ctx, mongoSpan)
	if err != nil {
		return fmt.Errorf("failed to insert reconcile span: %w", err)
	}

	return nil
}

// EndReconcileSpan updates end time and error for a span.
func (m *MongoStore) EndReconcileSpan(
	spanID string,
	endTime time.Time,
	durationMs int64,
	errMsg string,
) error {
	err := assert.AssertStringNotEmpty(spanID, "span ID")
	if err != nil {
		return err
	}

	filter := bson.M{"_id": spanID}
	update := bson.M{"$set": bson.M{
		"end_ts":      endTime,
		"duration_ms": durationMs,
		"error":       errMsg,
	}}

	_, err = m.spanCollection.UpdateOne(m.ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update reconcile span: %w", err)
	}

	return nil
}

// QueryReconcileSpans retrieves spans for a session.
func (m *MongoStore) QueryReconcileSpans(sessionID string) ([]ReconcileSpan, error) {
	err := assert.AssertStringNotEmpty(sessionID, "session ID")
	if err != nil {
		return nil, err
	}

	filter := bson.M{"session_id": sessionID}
	opts := options.Find().
		SetSort(bson.M{"start_ts": 1}).
		SetLimit(int64(maxQueryResults))

	cursor, err := m.spanCollection.Find(m.ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("span query failed: %w", err)
	}
	defer func() {
		closeErr := cursor.Close(m.ctx)
		if closeErr != nil {
			fmt.Printf("Warning: failed to close cursor: %v\n", closeErr)
		}
	}()

	spans := make([]ReconcileSpan, 0, 1000)
	count := 0
	maxResults := 10000

	for cursor.Next(m.ctx) && count < maxResults {
		var mongoSpan MongoReconcileSpan

		err = cursor.Decode(&mongoSpan)
		if err != nil {
			return nil, fmt.Errorf("span decode failed: %w", err)
		}

		span := ReconcileSpan{
			ID:                     mongoSpan.ID,
			SessionID:              mongoSpan.SessionID,
			ActorID:                mongoSpan.ActorID,
			StartTime:              mongoSpan.StartTime,
			Kind:                   mongoSpan.Kind,
			Namespace:              mongoSpan.Namespace,
			Name:                   mongoSpan.Name,
			TriggerUID:             mongoSpan.TriggerUID,
			TriggerResourceVersion: mongoSpan.TriggerResourceVersion,
			TriggerReason:          mongoSpan.TriggerReason,
			Error:                  mongoSpan.Error,
		}

		if mongoSpan.EndTime != nil {
			span.EndTime = *mongoSpan.EndTime
		}
		if mongoSpan.DurationMs != nil {
			span.DurationMs = *mongoSpan.DurationMs
		}

		spans = append(spans, span)
		count = count + 1
	}

	err = cursor.Err()
	if err != nil {
		return nil, fmt.Errorf("span cursor iteration failed: %w", err)
	}

	return spans, nil
}

// QueryOperationsByRange retrieves operations within sequence range.
func (m *MongoStore) QueryOperationsByRange(
	sessionID string,
	start, end int64,
) ([]Operation, error) {
	err := assert.AssertStringNotEmpty(sessionID, "session ID")
	if err != nil {
		return nil, err
	}

	err = assert.AssertInRange(int(start), 0, int(end), "start sequence")
	if err != nil {
		return nil, err
	}

	filter := bson.M{
		"session_id": sessionID,
		"sequence_number": bson.M{
			"$gte": start,
			"$lte": end,
		},
	}

	opts := options.Find().
		SetSort(bson.M{"sequence_number": 1}).
		SetLimit(int64(maxQueryResults))

	cursor, err := m.collection.Find(m.ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("range query failed: %w", err)
	}
	defer func() {
		closeErr := cursor.Close(m.ctx)
		if closeErr != nil {
			fmt.Printf("Warning: failed to close cursor: %v\n", closeErr)
		}
	}()

	return m.scanOperations(cursor)
}

// ListSessions returns all available sessions.
func (m *MongoStore) ListSessions() ([]SessionInfo, error) {
	pipeline := []bson.M{
		{
			"$group": bson.M{
				"_id":        "$session_id",
				"start_time": bson.M{"$min": "$timestamp"},
				"end_time":   bson.M{"$max": "$timestamp"},
				"op_count":   bson.M{"$sum": 1},
			},
		},
		{
			"$sort": bson.M{"start_time": -1},
		},
		{
			"$limit": maxQueryResults,
		},
	}

	cursor, err := m.collection.Aggregate(m.ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("session query failed: %w", err)
	}
	defer func() {
		closeErr := cursor.Close(m.ctx)
		if closeErr != nil {
			fmt.Printf("Warning: failed to close cursor: %v\n", closeErr)
		}
	}()

	sessions := make([]SessionInfo, 0, 100)
	count := 0
	maxSessions := 1000

	for cursor.Next(m.ctx) && count < maxSessions {
		var result struct {
			ID        string    `bson:"_id"`
			StartTime time.Time `bson:"start_time"`
			EndTime   time.Time `bson:"end_time"`
			OpCount   int64     `bson:"op_count"`
		}

		err = cursor.Decode(&result)
		if err != nil {
			return nil, fmt.Errorf("session decode failed: %w", err)
		}

		session := SessionInfo{
			SessionID: result.ID,
			StartTime: result.StartTime.Unix(),
			EndTime:   result.EndTime.Unix(),
			OpCount:   result.OpCount,
		}

		sessions = append(sessions, session)
		count = count + 1
	}

	return sessions, nil
}

// Close closes the MongoDB connection.
func (m *MongoStore) Close() error {
	if m.client != nil {
		return m.client.Disconnect(m.ctx)
	}
	return nil
}

// createIndexes creates necessary indexes for optimal query performance.
func (m *MongoStore) createIndexes() error {
	indexes := []mongo.IndexModel{
		{
			Keys: bson.M{"session_id": 1, "sequence_number": 1},
		},
		{
			Keys: bson.M{"session_id": 1},
		},
		{
			Keys: bson.M{"timestamp": 1},
		},
	}

	indexCount := 0
	maxIndexes := 10

	for indexCount < len(indexes) && indexCount < maxIndexes {
		_, err := m.collection.Indexes().CreateOne(m.ctx, indexes[indexCount])
		if err != nil {
			return fmt.Errorf("failed to create index %d: %w", indexCount, err)
		}
		indexCount = indexCount + 1
	}

	spanIndexes := []mongo.IndexModel{
		{
			Keys: bson.M{"session_id": 1, "start_ts": 1},
		},
		{
			Keys: bson.M{"trigger_uid": 1, "trigger_resource_version": 1},
		},
	}

	indexCount = 0
	for indexCount < len(spanIndexes) && indexCount < maxIndexes {
		_, err := m.spanCollection.Indexes().CreateOne(m.ctx, spanIndexes[indexCount])
		if err != nil {
			return fmt.Errorf("failed to create span index %d: %w", indexCount, err)
		}
		indexCount = indexCount + 1
	}

	return nil
}

// scanOperations converts MongoDB cursor to Operation structs.
func (m *MongoStore) scanOperations(cursor *mongo.Cursor) ([]Operation, error) {
	operations := make([]Operation, 0, 1000)
	count := 0
	maxResults := 10000

	for cursor.Next(m.ctx) && count < maxResults {
		var mongoOp MongoOperation

		err := cursor.Decode(&mongoOp)
		if err != nil {
			return nil, fmt.Errorf("decode failed: %w", err)
		}

		op := Operation{
			SessionID:       mongoOp.SessionID,
			SequenceNumber:  mongoOp.SequenceNumber,
			Timestamp:       mongoOp.Timestamp,
			OperationType:   OperationType(mongoOp.OperationType),
			ResourceKind:    mongoOp.ResourceKind,
			Namespace:       mongoOp.Namespace,
			Name:            mongoOp.Name,
			ResourceData:    mongoOp.ResourceData,
			Error:           mongoOp.Error,
			DurationMs:      mongoOp.DurationMs,
			ActorID:         mongoOp.ActorID,
			UID:             mongoOp.UID,
			ResourceVersion: mongoOp.ResourceVersion,
			Generation:      mongoOp.Generation,
			Verb:            mongoOp.Verb,
		}

		operations = append(operations, op)
		count = count + 1
	}

	return operations, nil
}
