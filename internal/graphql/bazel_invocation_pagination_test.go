package graphql

import (
	"context"
	"testing"
	"time"

	"entgo.io/contrib/entgql"
	"github.com/buildbarn/bb-portal/ent/gen/ent"
	"github.com/buildbarn/bb-portal/ent/gen/ent/enttest"
	"github.com/buildbarn/bb-portal/internal/database/dbauthservice"
	"github.com/buildbarn/bb-portal/internal/graphql/helpers"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
)

func TestFindBazelInvocationsSeekPagination(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:find-bazel-invocations?mode=memory&cache=shared&_fk=1")
	t.Cleanup(func() {
		_ = client.Close()
	})
	ctx := dbauthservice.NewContextWithDbAuthServiceBypass(context.Background())

	instanceName := client.InstanceName.Create().
		SetName("").
		SaveX(ctx)

	startedAtA := time.Date(2026, 3, 10, 15, 0, 0, 0, time.UTC)
	startedAtB := time.Date(2026, 3, 9, 15, 0, 0, 0, time.UTC)
	startedAtC := time.Date(2026, 3, 8, 15, 0, 0, 0, time.UTC)

	invocation1 := createTestInvocation(t, client, instanceName, startedAtA)
	invocation2 := createTestInvocation(t, client, instanceName, startedAtA)
	invocation3 := createTestInvocation(t, client, instanceName, startedAtB)
	invocation4 := createTestInvocation(t, client, instanceName, startedAtC)

	resolver := &queryResolver{&Resolver{client: client}}
	first := 2

	page1, err := resolver.FindBazelInvocations(ctx, nil, &first, nil, nil, nil, nil)
	require.NoError(t, err)
	require.Equal(t, 4, page1.TotalCount)
	require.Equal(t, []uuid.UUID{
		invocation2.InvocationID,
		invocation1.InvocationID,
	}, invocationIDs(page1))
	require.True(t, page1.PageInfo.HasNextPage)
	require.False(t, page1.PageInfo.HasPreviousPage)
	require.NotNil(t, page1.PageInfo.StartCursor)
	require.NotNil(t, page1.PageInfo.EndCursor)

	page2, err := resolver.FindBazelInvocations(ctx, page1.PageInfo.EndCursor, &first, nil, nil, nil, nil)
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{
		invocation3.InvocationID,
		invocation4.InvocationID,
	}, invocationIDs(page2))
	require.False(t, page2.PageInfo.HasNextPage)
	require.True(t, page2.PageInfo.HasPreviousPage)
	require.NotNil(t, page2.PageInfo.StartCursor)

	last := 2
	backwardPage, err := resolver.FindBazelInvocations(ctx, nil, nil, page2.PageInfo.StartCursor, &last, nil, nil)
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{
		invocation2.InvocationID,
		invocation1.InvocationID,
	}, invocationIDs(backwardPage))
	require.True(t, backwardPage.PageInfo.HasNextPage)
	require.False(t, backwardPage.PageInfo.HasPreviousPage)
	require.NotNil(t, backwardPage.PageInfo.StartCursor)
	require.NotNil(t, backwardPage.PageInfo.EndCursor)

	require.Equal(t, page1.PageInfo.StartCursor, backwardPage.PageInfo.StartCursor)
	require.Equal(t, page1.PageInfo.EndCursor, backwardPage.PageInfo.EndCursor)
}

func createTestInvocation(t *testing.T, client *ent.Client, instanceName *ent.InstanceName, startedAt time.Time) *ent.BazelInvocation {
	t.Helper()
	ctx := dbauthservice.NewContextWithDbAuthServiceBypass(context.Background())

	return client.BazelInvocation.Create().
		SetInvocationID(uuid.New()).
		SetStartedAt(startedAt).
		SetInstanceName(instanceName).
		SaveX(ctx)
}

func invocationIDs(conn *ent.BazelInvocationConnection) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(conn.Edges))
	for _, edge := range conn.Edges {
		if edge == nil || edge.Node == nil {
			continue
		}
		ids = append(ids, edge.Node.InvocationID)
	}
	return ids
}

func TestPaginationCursorsToUTCHandlesCompositeCursorValues(t *testing.T) {
	localTime := time.Date(2026, 3, 10, 9, 0, 0, 0, time.FixedZone("PDT", -7*60*60))
	cursor := &entgql.Cursor[int64]{
		ID:    42,
		Value: []any{localTime},
	}

	helpers.PaginationCursorsToUTC(cursor, nil)

	values, ok := cursor.Value.([]any)
	require.True(t, ok)
	require.Len(t, values, 1)

	startedAt, ok := values[0].(time.Time)
	require.True(t, ok)
	require.Equal(t, time.UTC, startedAt.Location())
	require.Equal(t, localTime.UTC(), startedAt)
}
