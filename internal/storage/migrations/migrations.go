package migrations

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Migration struct {
	Version     int
	Description string
	Up          func(ctx context.Context, db *mongo.Database) error
	Down        func(ctx context.Context, db *mongo.Database) error
}

type MigrationManager struct {
	db         *mongo.Database
	logger     *slog.Logger
	migrations []Migration
	mu         sync.RWMutex
}

func NewMigrationManager(db *mongo.Database, logger *slog.Logger) *MigrationManager {
	if logger == nil {
		logger = slog.Default()
	}
	return &MigrationManager{
		db:         db,
		logger:     logger,
		migrations: DefaultMigrations(),
	}
}

func (m *MigrationManager) Add(migration Migration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.migrations = append(m.migrations, migration)
	sort.Slice(m.migrations, func(i, j int) bool {
		return m.migrations[i].Version < m.migrations[j].Version
	})
}

func (m *MigrationManager) Up(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if err := m.ensureMigrationsCollection(ctx); err != nil {
		return fmt.Errorf("ensure migrations collection: %w", err)
	}

	applied, err := m.getAppliedVersions(ctx)
	if err != nil {
		return fmt.Errorf("get applied versions: %w", err)
	}

	for _, migration := range m.migrations {
		if applied[migration.Version] {
			m.logger.Debug("migration already applied", "version", migration.Version)
			continue
		}

		m.logger.Info("applying migration", "version", migration.Version, "description", migration.Description)

		if err := migration.Up(ctx, m.db); err != nil {
			m.logger.Error("migration failed", "version", migration.Version, "error", err)
			return fmt.Errorf("migration %d failed: %w", migration.Version, err)
		}

		if err := m.recordApplied(ctx, migration.Version, migration.Description); err != nil {
			return fmt.Errorf("record migration: %w", err)
		}

		m.logger.Info("migration applied", "version", migration.Version)
	}

	return nil
}

func (m *MigrationManager) Down(ctx context.Context, targetVersion int) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	applied, err := m.getAppliedVersions(ctx)
	if err != nil {
		return fmt.Errorf("get applied versions: %w", err)
	}

	for i := len(m.migrations) - 1; i >= 0; i-- {
		migration := m.migrations[i]
		if migration.Version <= targetVersion {
			break
		}
		if !applied[migration.Version] {
			continue
		}

		if migration.Down == nil {
			m.logger.Warn("migration has no down function", "version", migration.Version)
			continue
		}

		m.logger.Info("reverting migration", "version", migration.Version, "description", migration.Description)

		if err := migration.Down(ctx, m.db); err != nil {
			return fmt.Errorf("down migration %d failed: %w", migration.Version, err)
		}

		if err := m.removeRecord(ctx, migration.Version); err != nil {
			return fmt.Errorf("remove migration record: %w", err)
		}

		m.logger.Info("migration reverted", "version", migration.Version)
	}

	return nil
}

func (m *MigrationManager) CurrentVersion(ctx context.Context) (int, error) {
	applied, err := m.getAppliedVersions(ctx)
	if err != nil {
		return 0, err
	}

	maxVersion := 0
	for version := range applied {
		if version > maxVersion {
			maxVersion = version
		}
	}

	return maxVersion, nil
}

func (m *MigrationManager) ensureMigrationsCollection(ctx context.Context) error {
	migrationColl := m.db.Collection("_migrations")
	if _, err := migrationColl.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "version", Value: 1}},
		Options: options.Index().SetUnique(true),
	}); err != nil {
		return fmt.Errorf("create migrations index: %w", err)
	}
	return nil
}

func (m *MigrationManager) getAppliedVersions(ctx context.Context) (map[int]bool, error) {
	migrationColl := m.db.Collection("_migrations")
	cursor, err := migrationColl.Find(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("query migrations: %w", err)
	}
	defer cursor.Close(ctx)

	applied := make(map[int]bool)
	for cursor.Next(ctx) {
		var record struct {
			Version int `bson:"version"`
		}
		if err := cursor.Decode(&record); err != nil {
			continue
		}
		applied[record.Version] = true
	}

	return applied, cursor.Err()
}

func (m *MigrationManager) recordApplied(ctx context.Context, version int, description string) error {
	migrationColl := m.db.Collection("_migrations")
	_, err := migrationColl.InsertOne(ctx, bson.M{
		"version":     version,
		"description": description,
		"applied_at":  time.Now(),
	})
	return err
}

func (m *MigrationManager) removeRecord(ctx context.Context, version int) error {
	migrationColl := m.db.Collection("_migrations")
	_, err := migrationColl.DeleteOne(ctx, bson.M{"version": version})
	return err
}

func DefaultMigrations() []Migration {
	return []Migration{
		{
			Version:     1,
			Description: "Create initial indexes for memories collection",
			Up: func(ctx context.Context, db *mongo.Database) error {
				coll := db.Collection("memories")
				_, err := coll.Indexes().CreateMany(ctx, []mongo.IndexModel{
					{Keys: bson.D{{Key: "conversation_id", Value: 1}}},
					{Keys: bson.D{{Key: "type", Value: 1}}},
					{Keys: bson.D{{Key: "created_at", Value: -1}}},
					{Keys: bson.D{{Key: "importance", Value: -1}}},
					{Keys: bson.D{{Key: "tags", Value: 1}}},
				})
				if err != nil {
					return fmt.Errorf("create memories indexes: %w", err)
				}
				return nil
			},
			Down: func(ctx context.Context, db *mongo.Database) error {
				coll := db.Collection("memories")
				_, err := coll.Indexes().DropOne(ctx, "created_at_-1")
				if err != nil {
					return fmt.Errorf("drop memories index: %w", err)
				}
				return nil
			},
		},
		{
			Version:     2,
			Description: "Create indexes for beliefs collection",
			Up: func(ctx context.Context, db *mongo.Database) error {
				coll := db.Collection("beliefs")
				_, err := coll.Indexes().CreateMany(ctx, []mongo.IndexModel{
					{Keys: bson.D{{Key: "subject", Value: 1}}},
					{Keys: bson.D{{Key: "status", Value: 1}}},
					{Keys: bson.D{{Key: "confidence", Value: -1}}},
					{Keys: bson.D{{Key: "updated_at", Value: -1}}},
				})
				if err != nil {
					return fmt.Errorf("create beliefs indexes: %w", err)
				}
				return nil
			},
			Down: func(ctx context.Context, db *mongo.Database) error {
				return nil
			},
		},
		{
			Version:     3,
			Description: "Create indexes for mind sub-collections",
			Up: func(ctx context.Context, db *mongo.Database) error {
				collections := []struct {
					name string
					keys bson.D
				}{
					{"opinions", bson.D{{Key: "subject", Value: 1}}},
					{"preferences", bson.D{{Key: "scope", Value: 1}}},
					{"goals", bson.D{{Key: "status", Value: 1}, {Key: "priority", Value: -1}}},
					{"hypotheses", bson.D{{Key: "status", Value: 1}}},
					{"uncertainties", bson.D{{Key: "status", Value: 1}}},
					{"self_observations", bson.D{{Key: "subject", Value: 1}}},
					{"contradictions", bson.D{{Key: "status", Value: 1}}},
				}

				for _, c := range collections {
					coll := db.Collection(c.name)
					if _, err := coll.Indexes().CreateOne(ctx, mongo.IndexModel{Keys: c.keys}); err != nil {
						return fmt.Errorf("create index for %s: %w", c.name, err)
					}
				}
				return nil
			},
			Down: func(ctx context.Context, db *mongo.Database) error {
				return nil
			},
		},
		{
			Version:     4,
			Description: "Create indexes for messages, events, and audit collections",
			Up: func(ctx context.Context, db *mongo.Database) error {
				messageColl := db.Collection("messages")
				if _, err := messageColl.Indexes().CreateMany(ctx, []mongo.IndexModel{
					{Keys: bson.D{{Key: "conversation_id", Value: 1}, {Key: "timestamp", Value: -1}}},
				}); err != nil {
					return fmt.Errorf("create messages indexes: %w", err)
				}

				eventColl := db.Collection("events")
				if _, err := eventColl.Indexes().CreateMany(ctx, []mongo.IndexModel{
					{Keys: bson.D{{Key: "pipeline_run_id", Value: 1}}},
					{Keys: bson.D{{Key: "timestamp", Value: -1}}},
				}); err != nil {
					return fmt.Errorf("create events indexes: %w", err)
				}

				auditColl := db.Collection("memory_history")
				if _, err := auditColl.Indexes().CreateMany(ctx, []mongo.IndexModel{
					{Keys: bson.D{{Key: "entity_id", Value: 1}}},
					{Keys: bson.D{{Key: "timestamp", Value: -1}}},
				}); err != nil {
					return fmt.Errorf("create audit indexes: %w", err)
				}

				return nil
			},
			Down: func(ctx context.Context, db *mongo.Database) error {
				return nil
			},
		},
	}
}
