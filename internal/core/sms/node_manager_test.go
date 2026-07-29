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

// Create implements [MediaServerStorer].
func (t *TestMediaServerStorer) Create(context.Context, *MediaServer) error {
	panic("unimplemented")
}

// Delete implements [MediaServerStorer].
func (t *TestMediaServerStorer) Delete(context.Context, *MediaServer, ...orm.QueryOption) error {
	panic("unimplemented")
}

// List implements [MediaServerStorer].
func (t *TestMediaServerStorer) List(context.Context, *[]*MediaServer, orm.Pager, ...orm.QueryOption) (int64, error) {
	panic("unimplemented")
}

// Update implements [MediaServerStorer].
func (t *TestMediaServerStorer) Update(context.Context, *MediaServer, func(*MediaServer), ...orm.QueryOption) error {
	panic("unimplemented")
}

// Add implements MediaServerStorer.
func (t *TestMediaServerStorer) Add(context.Context, *MediaServer) error {
	panic("unimplemented")
}

// Del implements MediaServerStorer.
func (t *TestMediaServerStorer) Del(context.Context, *MediaServer, ...orm.QueryOption) error {
	panic("unimplemented")
}

// Edit implements MediaServerStorer.
func (t *TestMediaServerStorer) Edit(ctx context.Context, in *MediaServer, fn func(*MediaServer), args ...orm.QueryOption) error {
	fn(in)
	fmt.Println("edit status:", in.Status)
	return nil
}

// Find implements MediaServerStorer.
func (t *TestMediaServerStorer) Find(context.Context, *[]*MediaServer, orm.Pager, ...orm.QueryOption) (int64, error) {
	panic("unimplemented")
}

// Get implements MediaServerStorer.
func (t *TestMediaServerStorer) Get(context.Context, *MediaServer, ...orm.QueryOption) error {
	panic("unimplemented")
}

// MediaServer implements Storer.
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
