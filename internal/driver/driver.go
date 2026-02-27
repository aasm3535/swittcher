package driver

import "fmt"

type ProfileInfo struct {
	Email     string
	Plan      string
	AccountID string
}

type UsageStats struct {
	WeeklyPct *float64
	HourlyPct *float64
}

type AppDriver interface {
	ID() string
	DisplayName() string
	IsAvailable() bool
	Login(profileDir string) error
	Launch(profileDir string) error
	ProfileInfo(profileDir string) (ProfileInfo, error)
	Usage(profileDir string) (*UsageStats, error)
}

type Registry struct {
	order   []string
	drivers map[string]AppDriver
}

func NewRegistry() *Registry {
	return &Registry{
		order:   []string{},
		drivers: map[string]AppDriver{},
	}
}

func (r *Registry) Register(d AppDriver) error {
	id := d.ID()
	if _, exists := r.drivers[id]; exists {
		return fmt.Errorf("driver %q already registered", id)
	}
	r.drivers[id] = d
	r.order = append(r.order, id)
	return nil
}

func (r *Registry) Get(id string) (AppDriver, bool) {
	d, ok := r.drivers[id]
	return d, ok
}

func (r *Registry) All() []AppDriver {
	out := make([]AppDriver, 0, len(r.order))
	for _, id := range r.order {
		out = append(out, r.drivers[id])
	}
	return out
}
