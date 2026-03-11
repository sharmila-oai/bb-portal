package graphql

import (
	"context"
	"fmt"
	"time"

	"entgo.io/contrib/entgql"
	"entgo.io/ent/dialect/sql"
	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/errcode"
	"github.com/buildbarn/bb-portal/ent/gen/ent"
	"github.com/buildbarn/bb-portal/ent/gen/ent/bazelinvocation"
	"github.com/buildbarn/bb-portal/ent/gen/ent/predicate"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

const bazelInvocationInvalidPaginationCode = "INVALID_PAGINATION"

func validateSeekPagination(first, last *int) (err *gqlerror.Error) {
	switch {
	case first != nil && last != nil:
		err = &gqlerror.Error{
			Message: "Passing both `first` and `last` to paginate a connection is not supported.",
		}
	case first != nil && *first < 0:
		err = &gqlerror.Error{
			Message: "`first` on a connection cannot be less than zero.",
		}
		errcode.Set(err, bazelInvocationInvalidPaginationCode)
	case last != nil && *last < 0:
		err = &gqlerror.Error{
			Message: "`last` on a connection cannot be less than zero.",
		}
		errcode.Set(err, bazelInvocationInvalidPaginationCode)
	}
	return err
}

func hasCollectedField(ctx context.Context, path ...string) bool {
	fc := graphql.GetFieldContext(ctx)
	if fc == nil {
		return true
	}
	field := fc.Field
	oc := graphql.GetOperationContext(ctx)
walk:
	for _, name := range path {
		for _, collected := range graphql.CollectFields(oc, field.Selections, nil) {
			if collected.Alias == name {
				field = collected
				continue walk
			}
		}
		return false
	}
	return true
}

func validateFindBazelInvocationsOrder(orderBy *ent.BazelInvocationOrder) error {
	if orderBy == nil {
		return nil
	}

	field := "STARTED_AT"
	if orderBy.Field != nil {
		field = orderBy.Field.String()
	}

	direction := entgql.OrderDirectionDesc
	if orderBy.Direction != "" {
		direction = orderBy.Direction
	}

	if field != "STARTED_AT" || direction != entgql.OrderDirectionDesc {
		return fmt.Errorf("findBazelInvocations only supports orderBy { field: STARTED_AT, direction: DESC }")
	}

	return nil
}

func bazelInvocationCursorTime(cursor *entgql.Cursor[int64]) (time.Time, error) {
	if cursor == nil || cursor.Value == nil {
		return time.Time{}, fmt.Errorf("cursor is missing started_at")
	}

	switch value := cursor.Value.(type) {
	case time.Time:
		return value.UTC(), nil
	case *time.Time:
		if value == nil {
			return time.Time{}, fmt.Errorf("cursor is missing started_at")
		}
		return value.UTC(), nil
	case []any:
		if len(value) == 0 {
			return time.Time{}, fmt.Errorf("cursor is missing started_at")
		}
		switch startedAt := value[0].(type) {
		case time.Time:
			return startedAt.UTC(), nil
		case *time.Time:
			if startedAt == nil {
				return time.Time{}, fmt.Errorf("cursor is missing started_at")
			}
			return startedAt.UTC(), nil
		default:
			return time.Time{}, fmt.Errorf("unsupported started_at cursor value %T", value[0])
		}
	default:
		return time.Time{}, fmt.Errorf("unsupported cursor value %T", cursor.Value)
	}
}

func bazelInvocationCursorPredicate(cursor *entgql.Cursor[int64], direction entgql.OrderDirection) (predicate.BazelInvocation, error) {
	startedAt, err := bazelInvocationCursorTime(cursor)
	if err != nil {
		return nil, err
	}

	return func(selector *sql.Selector) {
		startedAtColumn := selector.C(bazelinvocation.FieldStartedAt)
		idColumn := selector.C(bazelinvocation.FieldID)

		var comparison *sql.Predicate
		if direction == entgql.OrderDirectionAsc {
			comparison = sql.Or(
				sql.GT(startedAtColumn, startedAt),
				sql.And(
					sql.EQ(startedAtColumn, startedAt),
					sql.GT(idColumn, cursor.ID),
				),
			)
		} else {
			comparison = sql.Or(
				sql.LT(startedAtColumn, startedAt),
				sql.And(
					sql.EQ(startedAtColumn, startedAt),
					sql.LT(idColumn, cursor.ID),
				),
			)
		}

		selector.Where(comparison)
	}, nil
}

func bazelInvocationCursorForNode(node *ent.BazelInvocation) ent.Cursor {
	return ent.Cursor{
		ID:    node.ID,
		Value: []any{node.StartedAt.UTC()},
	}
}

func reverseBazelInvocationNodes(nodes []*ent.BazelInvocation) {
	for left, right := 0, len(nodes)-1; left < right; left, right = left+1, right-1 {
		nodes[left], nodes[right] = nodes[right], nodes[left]
	}
}

func buildBazelInvocationConnection(
	nodes []*ent.BazelInvocation,
	after *entgql.Cursor[int64],
	before *entgql.Cursor[int64],
	first *int,
	last *int,
	totalCount *int,
) *ent.BazelInvocationConnection {
	conn := &ent.BazelInvocationConnection{
		Edges: make([]*ent.BazelInvocationEdge, 0, len(nodes)),
	}
	if totalCount != nil {
		conn.TotalCount = *totalCount
	}

	conn.PageInfo.HasPreviousPage = after != nil
	conn.PageInfo.HasNextPage = before != nil

	if first != nil && len(nodes) == *first+1 {
		conn.PageInfo.HasNextPage = true
		nodes = nodes[:len(nodes)-1]
	} else if last != nil && len(nodes) == *last+1 {
		conn.PageInfo.HasPreviousPage = true
		nodes = nodes[:len(nodes)-1]
	}

	if last != nil {
		reverseBazelInvocationNodes(nodes)
	}

	for _, node := range nodes {
		conn.Edges = append(conn.Edges, &ent.BazelInvocationEdge{
			Node:   node,
			Cursor: bazelInvocationCursorForNode(node),
		})
	}

	if len(conn.Edges) > 0 {
		conn.PageInfo.StartCursor = &conn.Edges[0].Cursor
		conn.PageInfo.EndCursor = &conn.Edges[len(conn.Edges)-1].Cursor
	}

	return conn
}
