package socketio

import (
	"sync"
	"testing"
	"time"
)

func TestConnectionSetDuplicate(t *testing.T) {
	io := New()
	defer io.Close()
	admin := io.Of("/admin")
	s1 := &Socket{Id: "user-1"}
	admin.socketJoinRoom("room-1", s1)
	// Join again - should not cause deadlock
	admin.socketJoinRoom("room-1", s1)

	done := make(chan bool)
	go func() {
		r := admin.To("room-1")
		if r == nil {
			t.Error("expected room not nil")
		}
		done <- true
	}()
	select {
	case <-done:
		t.Log("Success")
	case <-time.After(1 * time.Second):
		t.Fatal("STUCK! Deadlock detected at io.Of(\"/admin\").To(rName)!")
	}
}

func TestRoomChaining(t *testing.T) {
	io := New()
	defer io.Close()
	admin := io.Of("/admin")
	s1 := &Socket{Id: "user-1"}
	s2 := &Socket{Id: "user-2"}
	s3 := &Socket{Id: "user-3"}
	admin.socketJoinRoom("room-1", s1)
	admin.socketJoinRoom("room-2", s2)
	admin.socketJoinRoom("room-3", s3)

	r := admin.To("room-1").To("room-2")
	if r == nil {
		t.Fatal("expected r not nil")
	}
	if len(r.Sockets()) != 2 {
		t.Fatalf("expected 2 sockets, got %d", len(r.Sockets()))
	}

	// Chaining 3 rooms
	r3 := admin.To("room-1").To("room-2").To("room-3")
	if r3 == nil {
		t.Fatal("expected r3 not nil")
	}
	if len(r3.Sockets()) != 3 {
		t.Fatalf("expected 3 sockets, got %d", len(r3.Sockets()))
	}
	if r3.Name != "room-1_room-2_room-3" {
		t.Fatalf("expected name 'room-1_room-2_room-3', got '%s'", r3.Name)
	}
}

func TestRoomDeduplication(t *testing.T) {
	io := New()
	defer io.Close()
	admin := io.Of("/admin")
	s1 := &Socket{Id: "user-1"}

	// s1 joins both rooms
	admin.socketJoinRoom("room-1", s1)
	admin.socketJoinRoom("room-2", s1)

	r := admin.To("room-1").To("room-2")
	if len(r.Sockets()) != 1 {
		t.Fatalf("expected 1 deduplicated socket, got %d", len(r.Sockets()))
	}
}

func TestConcurrentTo(t *testing.T) {
	io := New()
	defer io.Close()
	admin := io.Of("/admin")
	s1 := &Socket{Id: "user-1"}
	admin.socketJoinRoom("room-1", s1)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := admin.To("room-1")
			if len(r.Sockets()) != 1 {
				t.Errorf("expected 1 socket, got %d", len(r.Sockets()))
			}
			r2 := r.To("room-2")
			if len(r2.Sockets()) != 1 {
				t.Errorf("expected 1 socket, got %d", len(r2.Sockets()))
			}
		}()
	}
	wg.Wait()
}
