package tools

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

var ScopeName = "dbf"

func concat(strs ...string) string {
	var builder strings.Builder
	for str := range slices.Values(strs) {
		builder.WriteString(str)
	}

	return builder.String()
}

func GetTestDatabase(bctx context.Context, t *testing.T) (*sqlx.DB, func()) {
	t.Helper()
	db, closer, err := MakeTestDatabase(bctx, uuid.New().String()[0:7])
	if err != nil {
		t.Fatal(err)
	}

	return db, func() {
		t.Helper()
		if err := closer(); err != nil {
			t.Fatal(err)
		}
	}
}

func MakeTestDatabase(bctx context.Context, name string) (*sqlx.DB, func() error, error) {
	ctx, cancel := context.WithCancel(bctx)
	dbName := concat(ScopeName, "_testing_", name)

	tdb, err := sqlx.ConnectContext(ctx, "postgres", concat("user=", ScopeName, " database=postgres sslmode=disable"))
	if err != nil {
		cancel()
		return nil, nil, err
	}
	_, _ = tdb.Exec(concat("CREATE DATABASE ", dbName))

	db, err := sqlx.ConnectContext(ctx, "postgres", concat("user=", ScopeName, " database=", dbName, " sslmode=disable"))
	if err != nil {
		cancel()
		return nil, nil, err
	}

	db.SetMaxOpenConns(128)
	db.SetMaxIdleConns(8)

	closer := func() error {
		cancel()
		errs := []error{
			db.Close(),
		}

		_, err = tdb.Exec("SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = $1;", dbName)
		errs = append(errs, fmt.Errorf("problem killing connections", err))

		_, err = tdb.Exec(concat("DROP DATABASE ", dbName))
		if perr, ok := err.(*pq.Error); ok && perr.Code == "3D000" {
			errs = append(errs, fmt.Errorf("warning: %w", err))
		} else {
			errs = append(errs, err)
		}

		return errors.Join(append(errs, tdb.Close())...)
	}

	return db, closer, nil
}
