package store

import (
	"context"
	"database/sql"
)

func lockCartRevision(ctx context.Context, tx *sql.Tx, accountID string) (int64, error) {
	if _, err := tx.ExecContext(ctx, `INSERT INTO cart_revisions (account_id, version) VALUES (?, 0)
		ON DUPLICATE KEY UPDATE version = version`, accountID); err != nil {
		return 0, err
	}
	var version int64
	err := tx.QueryRowContext(ctx, `SELECT version FROM cart_revisions WHERE account_id = ? FOR UPDATE`, accountID).Scan(&version)
	return version, err
}

func bumpCartRevision(ctx context.Context, tx *sql.Tx, accountID string) error {
	_, err := tx.ExecContext(ctx, `UPDATE cart_revisions SET version = version + 1 WHERE account_id = ?`, accountID)
	return err
}

func (s *MySQLStore) mutateCart(ctx context.Context, accountID string, apply func(*sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := lockCartRevision(ctx, tx, accountID); err != nil {
		return err
	}
	if err := apply(tx); err != nil {
		return err
	}
	if err := bumpCartRevision(ctx, tx, accountID); err != nil {
		return err
	}
	return tx.Commit()
}
