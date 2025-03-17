package metrics

import (
	"testing"
	"time"

	"github.com/tunnels-is/tunnels/coretwo/pkg/logger"
)

func TestNewRegistry(t *testing.T) {
	logger := logger.New(logger.DebugLevel, nil, false)
	registry := NewRegistry(logger, time.Second)
	if registry == nil {
		t.Error("NewRegistry returned nil")
	}
	if registry.metrics == nil {
		t.Error("metrics map is nil")
	}
}

func TestRegistry_RegisterCounter(t *testing.T) {
	logger := logger.New(logger.DebugLevel, nil, false)
	registry := NewRegistry(logger, time.Second)

	counter := registry.RegisterCounter("test_counter", "Test counter", map[string]string{"label": "value"})
	if counter == nil {
		t.Error("RegisterCounter returned nil")
	}

	// Test duplicate registration
	duplicate := registry.RegisterCounter("test_counter", "Duplicate", nil)
	if duplicate != nil {
		t.Error("Duplicate registration should return nil")
	}
}

func TestRegistry_RegisterGauge(t *testing.T) {
	logger := logger.New(logger.DebugLevel, nil, false)
	registry := NewRegistry(logger, time.Second)

	gauge := registry.RegisterGauge("test_gauge", "Test gauge", map[string]string{"label": "value"})
	if gauge == nil {
		t.Error("RegisterGauge returned nil")
	}

	// Test duplicate registration
	duplicate := registry.RegisterGauge("test_gauge", "Duplicate", nil)
	if duplicate != nil {
		t.Error("Duplicate registration should return nil")
	}
}

func TestRegistry_RegisterHistogram(t *testing.T) {
	logger := logger.New(logger.DebugLevel, nil, false)
	registry := NewRegistry(logger, time.Second)

	histogram := registry.RegisterHistogram("test_histogram", "Test histogram", map[string]string{"label": "value"})
	if histogram == nil {
		t.Error("RegisterHistogram returned nil")
	}

	// Test duplicate registration
	duplicate := registry.RegisterHistogram("test_histogram", "Duplicate", nil)
	if duplicate != nil {
		t.Error("Duplicate registration should return nil")
	}
}

func TestCounter_Operations(t *testing.T) {
	logger := logger.New(logger.DebugLevel, nil, false)
	registry := NewRegistry(logger, time.Second)

	counter := registry.RegisterCounter("test_counter", "Test counter", nil)
	if counter == nil {
		t.Fatal("Failed to register counter")
	}

	// Test increment
	counter.Inc()
	if counter.Value != 1 {
		t.Errorf("Expected counter value 1, got %f", counter.Value)
	}

	// Test add
	counter.Add(5)
	if counter.Value != 6 {
		t.Errorf("Expected counter value 6, got %f", counter.Value)
	}
}

func TestGauge_Operations(t *testing.T) {
	logger := logger.New(logger.DebugLevel, nil, false)
	registry := NewRegistry(logger, time.Second)

	gauge := registry.RegisterGauge("test_gauge", "Test gauge", nil)
	if gauge == nil {
		t.Fatal("Failed to register gauge")
	}

	// Test set
	gauge.Set(42.5)
	if gauge.Value != 42.5 {
		t.Errorf("Expected gauge value 42.5, got %f", gauge.Value)
	}
}

func TestHistogram_Operations(t *testing.T) {
	logger := logger.New(logger.DebugLevel, nil, false)
	registry := NewRegistry(logger, time.Second)

	histogram := registry.RegisterHistogram("test_histogram", "Test histogram", nil)
	if histogram == nil {
		t.Fatal("Failed to register histogram")
	}

	// Test add
	histogram.Add(1.5)
	if histogram.Value != 1.5 {
		t.Errorf("Expected histogram value 1.5, got %f", histogram.Value)
	}

	// Test bucket counting
	if histogram.buckets[1.5] != 1 {
		t.Errorf("Expected bucket count 1, got %d", histogram.buckets[1.5])
	}
}

func TestRegistry_StartStop(t *testing.T) {
	logger := logger.New(logger.DebugLevel, nil, false)
	registry := NewRegistry(logger, 100*time.Millisecond)

	// Register a test metric
	counter := registry.RegisterCounter("test_counter", "Test counter", nil)
	if counter == nil {
		t.Fatal("Failed to register counter")
	}

	// Start collection
	registry.Start()

	// Increment counter
	counter.Inc()

	// Wait for collection
	time.Sleep(150 * time.Millisecond)

	// Stop collection
	registry.Stop()
}

func TestMetricType_String(t *testing.T) {
	tests := []struct {
		metricType MetricType
		expected   string
	}{
		{CounterType, "counter"},
		{GaugeType, "gauge"},
		{HistogramType, "histogram"},
		{MetricType(999), "unknown"},
	}

	for _, test := range tests {
		if test.metricType.String() != test.expected {
			t.Errorf("Expected %s for metric type %v, got %s", test.expected, test.metricType, test.metricType.String())
		}
	}
}
