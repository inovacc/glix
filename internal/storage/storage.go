package storage

import (
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"
)

var (
	DirectoriesBucket  = []byte("directories")
	RepositoriesBucket = []byte("repositories")
	SchedulesBucket    = []byte("schedules")
)

type DB struct {
	db *bolt.DB
}

func Open(path string) (*DB, error) {
	db, err := bolt.Open(path, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Create buckets if they don't exist
	err = db.Update(func(tx *bolt.Tx) error {
		for _, bucket := range [][]byte{DirectoriesBucket, RepositoriesBucket, SchedulesBucket} {
			if _, err := tx.CreateBucketIfNotExists(bucket); err != nil {
				return fmt.Errorf("failed to create bucket: %w", err)
			}
		}

		return nil
	})
	if err != nil {
		if err := db.Close(); err != nil {
			return nil, err
		}
		return nil, err
	}

	return &DB{db: db}, nil
}

// Close closes the database
func (d *DB) Close() error {
	return d.db.Close()
}
