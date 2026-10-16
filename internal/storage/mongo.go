package storage

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/requiem/cognitive/internal/storage/migrations"
)

type MongoConfig struct {
	URI      string
	Database string
}

type MongoStore struct {
	client       *mongo.Client
	database     *mongo.Database
	logger       *slog.Logger
	isReplicaSet bool

	memoryCollection              *mongo.Collection
	beliefsCollection             *mongo.Collection
	opinionsCollection            *mongo.Collection
	preferencesCollection         *mongo.Collection
	valuesCollection              *mongo.Collection
	goalsCollection               *mongo.Collection
	hypothesesCollection          *mongo.Collection
	uncertaintiesCollection       *mongo.Collection
	selfObservationsCollection    *mongo.Collection
	contradictionsCollection      *mongo.Collection
	messagesCollection            *mongo.Collection
	eventsCollection              *mongo.Collection
	auditCollection               *mongo.Collection
	pipelineRunsCollection        *mongo.Collection
	memoryHistoryCollection       *mongo.Collection
	usersCollection               *mongo.Collection
	verificationTokensCollection  *mongo.Collection
	passwordResetTokensCollection *mongo.Collection
	evidenceCollection            *mongo.Collection
	claimsCollection              *mongo.Collection
	verificationPlansCollection   *mongo.Collection
	verificationResultsCollection *mongo.Collection
}

func NewMongoStore(ctx context.Context, cfg MongoConfig, logger *slog.Logger) (*MongoStore, error) {
	if logger == nil {
		logger = slog.Default()
	}

	clientOptions := options.Client().ApplyURI(cfg.URI)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, fmt.Errorf("connect to MongoDB: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx, nil); err != nil {
		return nil, fmt.Errorf("ping MongoDB: %w", err)
	}

	isReplica, err := detectReplicaSet(pingCtx, client)
	if err != nil {
		logger.Warn("failed to detect replica set status, assuming standalone", "error", err)
	}

	db := client.Database(cfg.Database)
	store := &MongoStore{
		client:                        client,
		database:                      db,
		logger:                        logger,
		isReplicaSet:                  isReplica,
		memoryCollection:              db.Collection("memories"),
		beliefsCollection:             db.Collection("beliefs"),
		opinionsCollection:            db.Collection("opinions"),
		preferencesCollection:         db.Collection("preferences"),
		valuesCollection:              db.Collection("values"),
		goalsCollection:               db.Collection("goals"),
		hypothesesCollection:          db.Collection("hypotheses"),
		uncertaintiesCollection:       db.Collection("uncertainties"),
		selfObservationsCollection:    db.Collection("self_observations"),
		contradictionsCollection:      db.Collection("contradictions"),
		messagesCollection:            db.Collection("messages"),
		eventsCollection:              db.Collection("events"),
		auditCollection:               db.Collection("audit"),
		pipelineRunsCollection:        db.Collection("pipeline_runs"),
		memoryHistoryCollection:       db.Collection("memory_history"),
		usersCollection:               db.Collection("users"),
		verificationTokensCollection:  db.Collection("verification_tokens"),
		passwordResetTokensCollection: db.Collection("password_reset_tokens"),
		evidenceCollection:            db.Collection("evidence_records"),
		claimsCollection:              db.Collection("claims"),
		verificationPlansCollection:   db.Collection("verification_plans"),
		verificationResultsCollection: db.Collection("verification_results"),
	}

	if err := store.createIndexes(ctx); err != nil {
		logger.Warn("failed to create some indexes", "error", err)
	}

	if err := store.RunMigrations(ctx); err != nil {
		logger.Warn("failed to run migrations", "error", err)
	}

	logger.Info("MongoDB store initialized", "database", cfg.Database)
	return store, nil
}

func (s *MongoStore) Close(ctx context.Context) error {
	return s.client.Disconnect(ctx)
}

func (s *MongoStore) createIndexes(ctx context.Context) error {
	indexes := map[string][]mongo.IndexModel{
		"memories": {
			{Keys: bson.D{{Key: "conversation_id", Value: 1}}},
			{Keys: bson.D{{Key: "type", Value: 1}}},
			{Keys: bson.D{{Key: "created_at", Value: -1}}},
			{Keys: bson.D{{Key: "importance", Value: -1}}},
			{Keys: bson.D{{Key: "tags", Value: 1}}},
		},
		"beliefs": {
			{Keys: bson.D{{Key: "subject", Value: 1}}},
			{Keys: bson.D{{Key: "status", Value: 1}}},
			{Keys: bson.D{{Key: "confidence", Value: -1}}},
			{Keys: bson.D{{Key: "updated_at", Value: -1}}},
		},
		"opinions": {
			{Keys: bson.D{{Key: "subject", Value: 1}}},
			{Keys: bson.D{{Key: "status", Value: 1}}},
		},
		"preferences": {
			{Keys: bson.D{{Key: "scope", Value: 1}}},
			{Keys: bson.D{{Key: "status", Value: 1}}},
		},
		"goals": {
			{Keys: bson.D{{Key: "status", Value: 1}}},
			{Keys: bson.D{{Key: "priority", Value: -1}}},
		},
		"hypotheses": {
			{Keys: bson.D{{Key: "status", Value: 1}}},
		},
		"uncertainties": {
			{Keys: bson.D{{Key: "status", Value: 1}}},
		},
		"contradictions": {
			{Keys: bson.D{{Key: "status", Value: 1}}},
		},
		"messages": {
			{Keys: bson.D{{Key: "conversation_id", Value: 1}, {Key: "timestamp", Value: -1}}},
		},
		"events": {
			{Keys: bson.D{{Key: "pipeline_run_id", Value: 1}}},
			{Keys: bson.D{{Key: "timestamp", Value: -1}}},
		},
		"memory_history": {
			{Keys: bson.D{{Key: "entity_id", Value: 1}}},
			{Keys: bson.D{{Key: "timestamp", Value: -1}}},
		},
		"users": {
			{Keys: bson.D{{Key: "email", Value: 1}}, Options: options.Index().SetUnique(true)},
		},
		"verification_tokens": {
			{Keys: bson.D{{Key: "token", Value: 1}}, Options: options.Index().SetUnique(true)},
			{Keys: bson.D{{Key: "expires_at", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)},
		},
		"password_reset_tokens": {
			{Keys: bson.D{{Key: "token", Value: 1}}, Options: options.Index().SetUnique(true)},
			{Keys: bson.D{{Key: "expires_at", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)},
		},
		"evidence_records": {
			{Keys: bson.D{{Key: "claim_id", Value: 1}}},
			{Keys: bson.D{{Key: "validity", Value: 1}}},
			{Keys: bson.D{{Key: "created_at", Value: -1}}},
		},
		"claims": {
			{Keys: bson.D{{Key: "subject", Value: 1}}},
			{Keys: bson.D{{Key: "status", Value: 1}}},
			{Keys: bson.D{{Key: "created_at", Value: -1}}},
		},
		"verification_plans": {
			{Keys: bson.D{{Key: "claim_id", Value: 1}}, Options: options.Index().SetUnique(true)},
		},
		"verification_results": {
			{Keys: bson.D{{Key: "claim_id", Value: 1}}, Options: options.Index().SetUnique(true)},
		},
	}

	for collName, models := range indexes {
		coll := s.database.Collection(collName)
		_, err := coll.Indexes().CreateMany(ctx, models)
		if err != nil {
			s.logger.Warn("failed to create indexes", "collection", collName, "error", err)
		}
	}
	return nil
}

func (s *MongoStore) Memory() MemoryStore {
	return &mongoMemoryStore{store: s}
}

func (s *MongoStore) Mind() MindStore {
	return &mongoMindStore{store: s}
}

func (s *MongoStore) Conversation() ConversationStore {
	return &mongoConversationStore{store: s}
}

func (s *MongoStore) Event() EventStore {
	return &mongoEventStore{store: s}
}

func (s *MongoStore) Audit() AuditStore {
	return &mongoAuditStore{store: s}
}

func (s *MongoStore) PipelineRun() PipelineRunStore {
	return &mongoPipelineRunStore{store: s}
}

func (s *MongoStore) Auth() AuthStore {
	return &mongoAuthStore{store: s}
}

func (s *MongoStore) Evidence() EvidenceStore {
	return &mongoEvidenceStore{store: s}
}

func (s *MongoStore) Claim() ClaimStore {
	return &mongoClaimStore{store: s}
}

func (s *MongoStore) Verification() VerificationStore {
	return &mongoVerificationStore{store: s}
}

func (s *MongoStore) Client() *mongo.Client {
	return s.client
}

func (s *MongoStore) Database() *mongo.Database {
	return s.database
}

func (s *MongoStore) StartTransaction(ctx context.Context) (context.Context, error) {
	if !s.isReplicaSet {
		return ctx, fmt.Errorf("transactions require replica set, running in standalone mode")
	}

	session, err := s.client.StartSession()
	if err != nil {
		return ctx, fmt.Errorf("start session: %w", err)
	}

	if err := session.StartTransaction(); err != nil {
		session.EndSession(ctx)
		return ctx, fmt.Errorf("start transaction: %w", err)
	}

	mongoCtx := mongo.NewSessionContext(ctx, session)
	return mongoCtx, nil
}

func (s *MongoStore) WithRetry(ctx context.Context, fn func(ctx context.Context) error) error {
	maxRetries := 3
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		err := fn(ctx)
		if err == nil {
			return nil
		}

		lastErr = err

		if attempt < maxRetries {
			s.logger.Warn("retrying operation",
				"attempt", attempt+1,
				"max_retries", maxRetries,
				"error", err,
			)
			time.Sleep(time.Duration(attempt+1) * 200 * time.Millisecond)
			continue
		}

		return err
	}

	return fmt.Errorf("exhausted retries after %d attempts, last error: %w", maxRetries+1, lastErr)
}

func (s *MongoStore) CommitTransaction(ctx context.Context) error {
	session := mongo.SessionFromContext(ctx)
	if session == nil {
		return nil
	}

	if err := session.CommitTransaction(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	session.EndSession(ctx)
	return nil
}

func (s *MongoStore) AbortTransaction(ctx context.Context) error {
	session := mongo.SessionFromContext(ctx)
	if session == nil {
		return nil
	}

	if err := session.AbortTransaction(ctx); err != nil {
		return fmt.Errorf("abort transaction: %w", err)
	}

	session.EndSession(ctx)
	return nil
}

func (s *MongoStore) RunMigrations(ctx context.Context) error {
	migrationMgr := migrations.NewMigrationManager(s.database, s.logger)
	if err := migrationMgr.Up(ctx); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	s.logger.Info("database migrations completed")
	return nil
}

func detectReplicaSet(ctx context.Context, client *mongo.Client) (bool, error) {
	var result bson.M
	if err := client.Database("admin").RunCommand(ctx, bson.D{{Key: "isMaster", Value: 1}}).Decode(&result); err != nil {
		return false, err
	}
	if setName, ok := result["setName"]; ok && setName != nil {
		return true, nil
	}
	if hosts, ok := result["hosts"]; ok && hosts != nil {
		return true, nil
	}
	return false, nil
}
