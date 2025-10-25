package pm_effectively_once

import (
	"context"
	"errors"

	"cloud.google.com/go/datastore"
)

type datastoreMutexer struct {
	kind     string
	dsClient *datastore.Client
}

func NewDatastoreMutexer(kind string, dsClient *datastore.Client) Mutexer {
	return &datastoreMutexer{kind: kind, dsClient: dsClient}
}

func (d *datastoreMutexer) RunInTx(ctx context.Context, deduplicateKey string, f func() error) error {
	key := datastore.NameKey(d.kind, deduplicateKey, nil)
	_, err := d.dsClient.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		if err := tx.Get(key, &struct{}{}); err == nil {
			// the event already processed
			return nil
		} else {
			if !errors.Is(err, datastore.ErrNoSuchEntity) {
				return err
			}
		}
		if _, err := tx.Put(key, &struct{}{}); err != nil {
			return err
		}

		return f()
	})
	return err
}
