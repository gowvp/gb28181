package ipcdb

import (
	"context"
	"testing"

	"github.com/gowvp/gb28181/internal/core/ipc"
	"github.com/ixugo/goddd/pkg/orm"
)

func TestDeviceGet(t *testing.T) {
	db, mock, err := generateMockDB()
	if err != nil {
		t.Fatal(err)
	}
	userDB := NewDevice(db)

	mock.ExpectQuery(`SELECT \* FROM "devices" WHERE id=\$1 (.+) LIMIT \$2`).WithArgs("jack", 1)
	var out ipc.Device
	if err := userDB.Get(context.Background(), &out, orm.Where("id=?", "jack")); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal("ExpectationsWereMet err:", err)
	}
}
