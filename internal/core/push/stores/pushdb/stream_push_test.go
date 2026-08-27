package pushdb

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestStreamPushGetByID(t *testing.T) {
	db, mock, err := generateMockDB()
	if err != nil {
		t.Fatal(err)
	}
	store := StreamPushDB{db: db}

	rows := sqlmock.NewRows([]string{"id", "app", "stream"}).AddRow("test-id", "live", "s1")
	mock.ExpectQuery(`SELECT \* FROM "stream_pushs" WHERE "stream_pushs"\."id" = \$1 LIMIT \$2`).
		WithArgs("test-id", 1).
		WillReturnRows(rows)
	if _, err := store.GetByID(context.Background(), "test-id"); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal("ExpectationsWereMet err:", err)
	}
}
