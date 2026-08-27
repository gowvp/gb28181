package proxydb

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestStreamProxyGetByID(t *testing.T) {
	db, mock, err := generateMockDB()
	if err != nil {
		t.Fatal(err)
	}
	store := StreamProxyDB{db: db}

	rows := sqlmock.NewRows([]string{"id", "app", "stream"}).AddRow("test-id", "live", "test-id")
	mock.ExpectQuery(`SELECT \* FROM "stream_proxys" WHERE "stream_proxys"\."id" = \$1 LIMIT \$2`).
		WithArgs("test-id", 1).
		WillReturnRows(rows)
	if _, err := store.GetByID(context.Background(), "test-id"); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal("ExpectationsWereMet err:", err)
	}
}
