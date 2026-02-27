package collectors

import (
	"cartographer-go-agent/configuration"
	"errors"
	"testing"
	"time"
)

func TestCollect_Success(t *testing.T) {
	cfg := &configuration.Config{}
	c := NewCollector("test", 1*time.Minute, cfg, func(config *configuration.Config) (interface{}, error) {
		return map[string]string{"key": "value"}, nil
	})

	data, err := c.Collect()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if data == nil {
		t.Fatal("expected data, got nil")
	}
	if c.LastStatus.Status != "ok" {
		t.Errorf("expected status 'ok', got %q", c.LastStatus.Status)
	}
	if c.LastStatus.Cached {
		t.Error("expected Cached=false on first run")
	}
	if c.LastStatus.LastRun == "" {
		t.Error("expected LastRun to be set")
	}
	if c.LastStatus.Error != "" {
		t.Errorf("expected no error in status, got %q", c.LastStatus.Error)
	}
}

func TestCollect_Cached(t *testing.T) {
	cfg := &configuration.Config{}
	calls := 0
	c := NewCollector("test", 1*time.Minute, cfg, func(config *configuration.Config) (interface{}, error) {
		calls++
		return "data", nil
	})

	// First call
	_, _ = c.Collect()
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}

	// Second call should use cache
	data, err := c.Collect()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call (cached), got %d", calls)
	}
	if data != "data" {
		t.Errorf("expected cached data, got %v", data)
	}
	if c.LastStatus.Status != "ok" {
		t.Errorf("expected status 'ok', got %q", c.LastStatus.Status)
	}
	if !c.LastStatus.Cached {
		t.Error("expected Cached=true on second run")
	}
}

func TestCollect_Error(t *testing.T) {
	cfg := &configuration.Config{}
	c := NewCollector("test", 1*time.Minute, cfg, func(config *configuration.Config) (interface{}, error) {
		return nil, errors.New("something broke")
	})

	data, err := c.Collect()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if data != nil {
		t.Errorf("expected nil data, got %v", data)
	}
	if c.LastStatus.Status != "error" {
		t.Errorf("expected status 'error', got %q", c.LastStatus.Status)
	}
	if c.LastStatus.Error != "something broke" {
		t.Errorf("expected error message 'something broke', got %q", c.LastStatus.Error)
	}
}

func TestCollect_Skipped(t *testing.T) {
	cfg := &configuration.Config{}
	c := NewCollector("test", 1*time.Minute, cfg, func(config *configuration.Config) (interface{}, error) {
		return nil, ErrCollectorSkipped
	})

	data, err := c.Collect()
	if !errors.Is(err, ErrCollectorSkipped) {
		t.Fatalf("expected ErrCollectorSkipped, got %v", err)
	}
	if data != nil {
		t.Errorf("expected nil data, got %v", data)
	}
	if c.LastStatus.Status != "skipped" {
		t.Errorf("expected status 'skipped', got %q", c.LastStatus.Status)
	}
}

func TestCollect_PanicRecovery(t *testing.T) {
	cfg := &configuration.Config{}
	c := NewCollector("test_panic", 1*time.Minute, cfg, func(config *configuration.Config) (interface{}, error) {
		panic("unexpected nil pointer")
	})

	data, err := c.Collect()
	if err == nil {
		t.Fatal("expected error from panic recovery, got nil")
	}
	if data != nil {
		t.Errorf("expected nil data after panic, got %v", data)
	}
	if c.LastStatus.Status != "error" {
		t.Errorf("expected status 'error' after panic, got %q", c.LastStatus.Status)
	}
	expected := "collector panicked: unexpected nil pointer"
	if c.LastStatus.Error != expected {
		t.Errorf("expected error %q, got %q", expected, c.LastStatus.Error)
	}
}

func TestCollect_DurationTracked(t *testing.T) {
	cfg := &configuration.Config{}
	c := NewCollector("test", 1*time.Minute, cfg, func(config *configuration.Config) (interface{}, error) {
		time.Sleep(10 * time.Millisecond)
		return "data", nil
	})

	_, _ = c.Collect()
	if c.LastStatus.Duration < 10 {
		t.Errorf("expected duration >= 10ms, got %.2fms", c.LastStatus.Duration)
	}
}
