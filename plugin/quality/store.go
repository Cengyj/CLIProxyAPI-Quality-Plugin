package quality

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	bolt "go.etcd.io/bbolt"
)

type Store struct {
	db       *bolt.DB
	migrated bool
}

var buckets = []string{"meta", "runs", "rules", "audit", "usage", "idempotency", "presets"}

func OpenStore(dir string, _ int64) (*Store, error) {
	if e := os.MkdirAll(dir, 0700); e != nil {
		return nil, e
	}
	db, e := bolt.Open(filepath.Join(dir, "quality.db"), 0600, &bolt.Options{Timeout: time.Nanosecond})
	if e != nil {
		return nil, fmt.Errorf("open plugin store (single instance required): %w", e)
	}
	s := &Store{db: db}
	e = db.Update(func(tx *bolt.Tx) error {
		for _, b := range buckets {
			if _, e := tx.CreateBucketIfNotExists([]byte(b)); e != nil {
				return e
			}
		}
		b := tx.Bucket([]byte("meta"))
		v := b.Get([]byte("schema"))
		if v != nil && string(v) != "1" && string(v) != "2" {
			return fmt.Errorf("unsupported plugin store schema")
		}
		if string(v) == "1" {
			if err := migrateManualOnly(tx); err != nil {
				return err
			}
			s.migrated = true
		}
		return b.Put([]byte("schema"), []byte("2"))
	})
	if e != nil {
		return nil, errors.Join(e, db.Close())
	}
	return s, nil
}
func (s *Store) Close() error { return s.db.Close() }
func (s *Store) Put(bucket, key string, v any) error {
	return s.Write(func(tx *bolt.Tx) error { return put(tx, bucket, key, v) })
}
func (s *Store) Write(fn func(*bolt.Tx) error) error {
	if err := s.db.Update(fn); err != nil {
		return fmt.Errorf("unavailable: write plugin state: %w", err)
	}
	return nil
}
func put(tx *bolt.Tx, bucket, key string, v any) error {
	raw, e := json.Marshal(v)
	if e != nil {
		return e
	}
	return tx.Bucket([]byte(bucket)).Put([]byte(key), raw)
}
func (s *Store) Get(bucket, key string, v any) error {
	return s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket)).Get([]byte(key))
		if b == nil {
			return os.ErrNotExist
		}
		return json.Unmarshal(b, v)
	})
}
func (s *Store) All(bucket string, fn func([]byte) error) error {
	err := s.db.View(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(bucket)).ForEach(func(k, v []byte) error { return fn(v) })
	})
	if err != nil {
		return fmt.Errorf("unavailable: read plugin state: %w", err)
	}
	return nil
}

// Prune only terminal history; actions and rule ownership are never expired.
func (s *Store) Prune(before time.Time, keep map[string]bool, cutoffs map[string]time.Time) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		for _, bucket := range []string{"runs", "usage", "audit"} {
			b := tx.Bucket([]byte(bucket))
			c := b.Cursor()
			for k, v := c.First(); k != nil; k, v = c.Next() {
				if bucket == "runs" && keep[string(k)] {
					continue
				}
				cutoff := before
				if bucket == "runs" {
					if custom, ok := cutoffs[string(k)]; ok {
						cutoff = custom
					}
				}
				var ts time.Time
				switch bucket {
				case "runs":
					var r Run
					if e := json.Unmarshal(v, &r); e != nil {
						return e
					}
					if r.CompletedAt != nil {
						ts = *r.CompletedAt
					}
				case "usage":
					var r UsageEvent
					if e := json.Unmarshal(v, &r); e != nil {
						return e
					}
					ts = r.Time
				case "audit":
					var r Audit
					if e := json.Unmarshal(v, &r); e != nil {
						return e
					}
					ts = r.Time
					if custom, ok := cutoffs[r.RunID]; ok {
						cutoff = custom
					}
				}
				if !ts.IsZero() && ts.Before(cutoff) {
					if e := c.Delete(); e != nil {
						return e
					}
				}
			}
		}
		return nil
	})
}
