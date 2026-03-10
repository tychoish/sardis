package main

import (
	"context"
	"database/sql"

	"github.com/tychoish/grip"
	_ "gitub.com/mattn/go-sqlite3"
)

func main() {
	db, err := sql.Open("sqlite3", "../../../fasolaminutes_parsing/minutes.db")
	grip.EmergencyPanic(err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db.ExecContext(ctx, "SELECT * FROM leaders;")
}
