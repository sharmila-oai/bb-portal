package integrationtest

import (
	"context"
	"os"
	"testing"

	"github.com/buildbarn/bb-portal/internal/api/http/bepuploader"
	"github.com/buildbarn/bb-portal/pkg/proto/configuration/bb_portal"
	"github.com/buildbarn/bb-portal/test/testutils"
	"github.com/buildbarn/bb-storage/pkg/proto/configuration/auth"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace/noop"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestStoreIncompleteProgressLogsKnob(t *testing.T) {
	t.Run("DefaultsToEnabled", func(t *testing.T) {
		count := uploadAndCountIncompleteLogs(t, nil)
		require.Positive(t, count)
	})

	t.Run("CanBeDisabled", func(t *testing.T) {
		disabled := false
		count := uploadAndCountIncompleteLogs(t, &disabled)
		require.Equal(t, 0, count)
	})
}

func uploadAndCountIncompleteLogs(t *testing.T, storeIncompleteProgressLogs *bool) int {
	t.Helper()

	db := testutils.SetupTestDB(t, dbProvider)
	config := &bb_portal.ApplicationConfiguration{
		InstanceNameAuthorizer: &auth.AuthorizerConfiguration{
			Policy: &auth.AuthorizerConfiguration_Allow{},
		},
		BesServiceConfiguration: &bb_portal.BuildEventStreamService{
			SaveDataLevel: &bb_portal.BuildEventStreamService_SaveDataLevel{
				Level: &bb_portal.BuildEventStreamService_SaveDataLevel_Basic{
					Basic: &emptypb.Empty{},
				},
			},
			StoreIncompleteProgressLogs: storeIncompleteProgressLogs,
		},
	}

	uploader, err := bepuploader.NewBepUploader(db, config, nil, nil, noop.NewTracerProvider(), uuid.NewRandom)
	require.NoError(t, err)

	file, err := os.Open(bepFolderPath + "/" + successfulBazelBuild.filename)
	require.NoError(t, err)
	defer file.Close()

	_, _, err = uploader.RecordEventNdjsonFile(context.Background(), file)
	require.NoError(t, err)

	count, err := db.Ent().IncompleteBuildLog.Query().Count(context.Background())
	require.NoError(t, err)
	return count
}
