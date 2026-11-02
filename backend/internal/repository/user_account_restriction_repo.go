package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type userAccountRestrictionRepository struct{ db *sql.DB }

func NewUserAccountRestrictionRepository(db *sql.DB) service.UserAccountRestrictionRepository {
	return &userAccountRestrictionRepository{db: db}
}

func (r *userAccountRestrictionRepository) GetByUserID(ctx context.Context, userID int64) (service.UserAccountRestrictions, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT group_id, account_id FROM user_group_account_restrictions WHERE user_id = $1 ORDER BY group_id, account_id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(service.UserAccountRestrictions)
	for rows.Next() {
		var groupID, accountID int64
		if err := rows.Scan(&groupID, &accountID); err != nil {
			return nil, err
		}
		result[groupID] = append(result[groupID], accountID)
	}
	return result, rows.Err()
}

func (r *userAccountRestrictionRepository) Replace(ctx context.Context, userID, groupID int64, accountIDs []int64) error {
	if userID <= 0 || groupID <= 0 || len(accountIDs) > 1000 {
		return service.ErrInvalidAccountRestriction
	}
	seen := make(map[int64]bool, len(accountIDs))
	for _, id := range accountIDs {
		if id <= 0 || seen[id] {
			return service.ErrInvalidAccountRestriction
		}
		seen[id] = true
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Serializes concurrent replacements and rejects soft-deleted users/groups.
	var id int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM users WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`, userID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrUserNotFound
	}
	if err != nil {
		return err
	}
	err = tx.QueryRowContext(ctx, `SELECT id FROM groups WHERE id = $1 AND deleted_at IS NULL FOR SHARE`, groupID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrGroupNotFound
	}
	if err != nil {
		return err
	}
	if len(accountIDs) > 0 {
		rows, err := tx.QueryContext(ctx, `SELECT a.id FROM accounts a JOIN account_groups ag ON ag.account_id = a.id WHERE ag.group_id = $1 AND a.id = ANY($2) AND a.deleted_at IS NULL FOR SHARE OF a, ag`, groupID, pq.Array(accountIDs))
		if err != nil {
			return err
		}
		count := 0
		for rows.Next() {
			count++
		}
		readErr := rows.Err()
		rows.Close()
		if readErr != nil {
			return readErr
		}
		if count != len(accountIDs) {
			return service.ErrInvalidAccountRestriction
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM user_group_account_restrictions WHERE user_id = $1 AND group_id = $2`, userID, groupID); err != nil {
		return err
	}
	if len(accountIDs) > 0 {
		if _, err := tx.ExecContext(ctx, `INSERT INTO user_group_account_restrictions (user_id, group_id, account_id) SELECT $1, $2, unnest($3::bigint[])`, userID, groupID, pq.Array(accountIDs)); err != nil {
			return err
		}
	}
	return tx.Commit()
}
