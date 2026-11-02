package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUserAccountRestrictionReplaceValidationAndAtomicity(t *testing.T) {
	for _, scenario := range []string{"save", "clear", "foreign_account", "missing_user", "missing_group", "insert_failure"} {
		t.Run(scenario, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			repo := NewUserAccountRestrictionRepository(db)
			ids := []int64{11}
			mock.ExpectBegin()
			user := mock.ExpectQuery("SELECT id FROM users.*deleted_at IS NULL FOR UPDATE").WithArgs(int64(7))
			if scenario == "missing_user" {
				user.WillReturnError(sql.ErrNoRows)
				mock.ExpectRollback()
			} else {
				user.WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(7))
				group := mock.ExpectQuery("SELECT id FROM groups.*deleted_at IS NULL FOR SHARE").WithArgs(int64(1))
				if scenario == "missing_group" {
					group.WillReturnError(sql.ErrNoRows)
					mock.ExpectRollback()
				} else {
					group.WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
					if scenario == "clear" {
						ids = []int64{}
					} else {
						rows := sqlmock.NewRows([]string{"id"})
						if scenario != "foreign_account" {
							rows.AddRow(11)
						}
						mock.ExpectQuery("SELECT a.id FROM accounts a JOIN account_groups.*a.deleted_at IS NULL FOR SHARE OF a, ag").WithArgs(int64(1), "{11}").WillReturnRows(rows)
					}
					if scenario == "foreign_account" {
						mock.ExpectRollback()
					} else {
						mock.ExpectExec("DELETE FROM user_group_account_restrictions WHERE user_id = \\$1 AND group_id = \\$2").WithArgs(int64(7), int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
						if scenario != "clear" {
							insert := mock.ExpectExec("INSERT INTO user_group_account_restrictions").WithArgs(int64(7), int64(1), "{11}")
							if scenario == "insert_failure" {
								insert.WillReturnError(errors.New("write failed"))
							} else {
								insert.WillReturnResult(sqlmock.NewResult(0, 1))
							}
						}
						if scenario == "insert_failure" {
							mock.ExpectRollback()
						} else {
							mock.ExpectCommit()
						}
					}
				}
			}
			err = repo.Replace(context.Background(), 7, 1, ids)
			if scenario == "save" || scenario == "clear" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestUserAccountRestrictionRejectsInvalidInputBeforeWrite(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewUserAccountRestrictionRepository(db)
	for _, ids := range [][]int64{{0}, {-1}, {11, 11}, make([]int64, 1001)} {
		require.ErrorIs(t, repo.Replace(context.Background(), 7, 1, ids), service.ErrInvalidAccountRestriction)
	}
	require.NoError(t, mock.ExpectationsWereMet())
}
