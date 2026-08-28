package sms

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ixugo/goddd/pkg/orm"
)

var (
	_ Storer            = (*TestStorer)(nil)
	_ MediaServerStorer = (*TestMediaServerStorer)(nil)
)

type (
	TestStorer            struct{}
	TestMediaServerStorer struct{}
)

func (t *TestMediaServerStorer) WithTx(orm.Tx) (MediaServerStorer, error) {
	panic("unimplemented")
}

func (t *TestMediaServerStorer) Create(context.Context, *MediaServer) error {
	panic("unimplemented")
}

func (t *TestMediaServerStorer) Delete(context.Context, *MediaServer) error {
	panic("unimplemented")
}

func (t *TestMediaServerStorer) List(context.Context, *FindMediaServerInput) ([]*MediaServer, int64, error) {
	panic("unimplemented")
}

func (t *TestMediaServerStorer) Update(_ context.Context, in *MediaServer, fn func(*MediaServer) error) error {
	if err := fn(in); err != nil {
		return err
	}
	fmt.Println("edit status:", in.Status)
	return nil
}

func (t *TestMediaServerStorer) GetByID(context.Context, string) (*MediaServer, error) {
	panic("unimplemented")
}

func (t *TestStorer) Begin() (orm.Tx, error) {
	panic("unimplemented")
}

func (t *TestStorer) MediaServer() MediaServerStorer {
	return &TestMediaServerStorer{}
}

func TestKeepalvie(t *testing.T) {
	var storer TestStorer
	nm := NewNodeManager(&storer)
	nm.cacheServers.Store("local", &WarpMediaServer{
		LastUpdatedAt: time.Now(),
	})
	time.Sleep(time.Second)
	nm.Keepalive("local")
	time.Sleep(25 * time.Second)
	nm.Keepalive("local")
	time.Sleep(5 * time.Second)
	// edit status: true
	// edit status: false
	// edit status: true
}
