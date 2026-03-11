package helpers

import (
	"time"

	"entgo.io/contrib/entgql"
)

// StringSliceArrayToPointerArray takes an array of strings and returns an array of string pointers
func StringSliceArrayToPointerArray(strings []string) []*string {
	result := make([]*string, len(strings))
	for i, str := range strings {
		result[i] = &str
	}
	return result
}

func normalizeCursorValueToUTC(value any) any {
	switch v := value.(type) {
	case time.Time:
		return v.UTC()
	case *time.Time:
		if v == nil {
			return v
		}
		ut := v.UTC()
		return &ut
	case []any:
		values := make([]any, len(v))
		for i, item := range v {
			values[i] = normalizeCursorValueToUTC(item)
		}
		return values
	default:
		return value
	}
}

func paginationCursorToUTC(cursor *entgql.Cursor[int64]) {
	if cursor == nil || cursor.Value == nil {
		return
	}
	cursor.Value = normalizeCursorValueToUTC(cursor.Value)
}

// PaginationCursorsToUTC converts pagination cursors that consist of
// timestamps to UTC instead of local time. When the backend sends the cursors
// to the frontend, they are in UTC. However, when the frontend sends them
// back, they are interpreted as local time. This causes issues since Sqlite
// cannot handle comparisons between timestamps in different timezones.
func PaginationCursorsToUTC(after, before *entgql.Cursor[int64]) {
	paginationCursorToUTC(after)
	paginationCursorToUTC(before)
}
