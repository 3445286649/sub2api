package repository

import (
	"context"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestChannelMonitorV3RejectUnavailableGroup(t *testing.T) {
	client, mock := newOllamaCloudUsageRepositoryTestClient(t)
	repo := &channelMonitorRepository{client: client}
	id := int64(88)
	mock.ExpectQuery(`SELECT .* FROM "groups" WHERE .*`).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	err := repo.Create(context.Background(), &service.ChannelMonitor{V3GroupID: &id})
	require.ErrorContains(t, err, "V3 statistics group is unavailable")
	require.NoError(t, mock.ExpectationsWereMet())
}
