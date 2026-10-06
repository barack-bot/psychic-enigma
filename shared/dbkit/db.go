package dbkit

import (
	"context"
	"database/sql"
)

func Ping(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return sql.ErrConnDone
	}
	return db.PingContext(ctx)
}
