package driver

import "testing"

type stubDriver struct {
	id   string
	name string
}

func (s *stubDriver) ID() string                                    { return s.id }
func (s *stubDriver) DisplayName() string                           { return s.name }
func (s *stubDriver) IsAvailable() bool                             { return true }
func (s *stubDriver) Login(string) error                            { return nil }
func (s *stubDriver) Launch(string) error                           { return nil }
func (s *stubDriver) ProfileInfo(string) (ProfileInfo, error)       { return ProfileInfo{}, nil }
func (s *stubDriver) Usage(string) (*UsageStats, error)             { return nil, nil }

func TestNewRegistryIsEmpty(t *testing.T) {
	r := NewRegistry()
	if got := len(r.All()); got != 0 {
		t.Fatalf("expected empty registry, got %d drivers", got)
	}
}

func TestRegisterAndGet(t *testing.T) {
	r := NewRegistry()
	d := &stubDriver{id: "claude", name: "Claude"}
	if err := r.Register(d); err != nil {
		t.Fatalf("register failed: %v", err)
	}

	got, ok := r.Get("claude")
	if !ok {
		t.Fatal("expected Get to find registered driver")
	}
	if got.ID() != "claude" {
		t.Fatalf("expected id claude, got %q", got.ID())
	}
}

func TestGetMissing(t *testing.T) {
	r := NewRegistry()
	_, ok := r.Get("nonexistent")
	if ok {
		t.Fatal("expected Get to return false for missing driver")
	}
}

func TestRegisterDuplicateFails(t *testing.T) {
	r := NewRegistry()
	d := &stubDriver{id: "codex", name: "Codex"}
	if err := r.Register(d); err != nil {
		t.Fatalf("first register failed: %v", err)
	}
	if err := r.Register(d); err == nil {
		t.Fatal("expected error when registering duplicate driver")
	}
}

func TestAllPreservesInsertionOrder(t *testing.T) {
	r := NewRegistry()
	ids := []string{"claude", "codex", "gemini"}
	for _, id := range ids {
		if err := r.Register(&stubDriver{id: id, name: id}); err != nil {
			t.Fatalf("register %s failed: %v", id, err)
		}
	}

	all := r.All()
	if len(all) != 3 {
		t.Fatalf("expected 3 drivers, got %d", len(all))
	}
	for i, id := range ids {
		if all[i].ID() != id {
			t.Fatalf("expected driver[%d] = %q, got %q", i, id, all[i].ID())
		}
	}
}
