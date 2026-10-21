//go:build unit

package usagestats

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestKeyUsageBucketsHourAnchorsAtStart(t *testing.T) {
	start := time.Date(2026, 10, 20, 15, 42, 0, 0, time.UTC)
	starts, ends := KeyUsageBuckets(start, start.Add(24*time.Hour), KeyUsageGranularityHour, nil)
	require.Len(t, starts, 24)
	require.Equal(t, start, starts[0])
	require.Equal(t, start.Add(24*time.Hour), ends[23])
	require.Equal(t, 42, starts[5].Minute())
}

func TestKeyUsageBucketsDayFollowsLocationAndDST(t *testing.T) {
	ny, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)
	start := time.Date(2026, 3, 7, 0, 0, 0, 0, ny)
	starts, ends := KeyUsageBuckets(start, time.Date(2026, 3, 10, 0, 0, 0, 0, ny), KeyUsageGranularityDay, ny)
	require.Len(t, starts, 3)
	require.Equal(t, 23*time.Hour, ends[1].Sub(starts[1]))
	require.Equal(t, 0, ends[1].In(ny).Hour())
}

func TestKeyUsageBucketsDayTruncatesPartialEdges(t *testing.T) {
	sh := time.FixedZone("CST", 8*3600)
	start := time.Date(2026, 10, 15, 12, 0, 0, 0, sh)
	starts, ends := KeyUsageBuckets(start, time.Date(2026, 10, 18, 6, 0, 0, 0, sh), KeyUsageGranularityDay, sh)
	require.Len(t, starts, 4)
	require.Equal(t, 12*time.Hour, ends[0].Sub(starts[0]))
	require.Equal(t, 6*time.Hour, ends[3].Sub(starts[3]))
}

func TestKeyUsageBucketsDisabled(t *testing.T) {
	starts, _ := KeyUsageBuckets(time.Now(), time.Now().Add(time.Hour), "", nil)
	require.Nil(t, starts)
}
