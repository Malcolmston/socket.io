package redis_test

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/malcolmston/socketio/redis"
)

func TestNewWithAuthAndSelect(t *testing.T) {
	fr := newFakeRedis(t)
	defer fr.close()

	// The fake server answers AUTH and SELECT with +OK (its default case), so a
	// broadcaster configured with a password and DB connects cleanly, exercising
	// connect()'s AUTH/SELECT branches and command().
	b, err := redis.New(redis.Options{
		Addr:     fr.addr(),
		Channel:  "t",
		Password: "secret",
		DB:       3,
	})
	if err != nil {
		t.Fatalf("New with auth/select: %v", err)
	}
	defer b.Close()

	if err := b.Publish([]byte("x")); err != nil {
		t.Fatalf("Publish: %v", err)
	}
}

func TestNewDialError(t *testing.T) {
	wantErr := errors.New("dial boom")
	_, err := redis.New(redis.Options{
		Addr: "unused",
		Dial: func(string) (net.Conn, error) { return nil, wantErr },
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("New err = %v, want %v", err, wantErr)
	}
}

func TestNewSecondDialError(t *testing.T) {
	fr := newFakeRedis(t)
	defer fr.close()

	// The first (pub) dial succeeds; the second (sub) dial fails. New must close
	// the pub connection it already opened and return the error.
	calls := 0
	_, err := redis.New(redis.Options{
		Addr:    fr.addr(),
		Channel: "t",
		Dial: func(addr string) (net.Conn, error) {
			calls++
			if calls == 2 {
				return nil, errors.New("second dial fails")
			}
			return net.Dial("tcp", addr)
		},
	})
	if err == nil {
		t.Fatal("expected an error when the sub connection fails to dial")
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	fr := newFakeRedis(t)
	defer fr.close()

	b, err := redis.New(redis.Options{Addr: fr.addr(), Channel: "t"})
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	// A second Close is a no-op and returns nil.
	if err := b.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestPublishAfterServerGone(t *testing.T) {
	fr := newFakeRedis(t)
	b, err := redis.New(redis.Options{Addr: fr.addr(), Channel: "t"})
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	// Tearing the server down and closing the broadcaster's connections makes a
	// subsequent Publish fail rather than hang.
	fr.close()
	_ = b.Close()
	time.Sleep(20 * time.Millisecond)
	if err := b.Publish([]byte("nope")); err == nil {
		t.Fatal("expected Publish to error after the connection is closed")
	}
}
