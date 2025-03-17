package metrics

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/tunnels-is/tunnels/coretwo/pkg/logger"
)

// MetricType represents the type of metric
type MetricType int

const (
	// CounterType represents a monotonically increasing metric
	CounterType MetricType = iota
	// GaugeType represents a metric that can increase and decrease
	GaugeType
	// HistogramType represents a metric that tracks the distribution of values
	HistogramType
)

// Metric represents a single metric
type Metric struct {
	Name        string            `json:"name"`
	Type        MetricType        `json:"type"`
	Description string            `json:"description"`
	Labels      map[string]string `json:"labels,omitempty"`
	Value       interface{}       `json:"value,omitempty"`
	LastUpdate  time.Time         `json:"last_update"`
	mu          sync.Mutex
}

// Counter represents a counter metric
type Counter struct {
	*Metric
	value uint64
}

// Gauge represents a gauge metric
type Gauge struct {
	*Metric
	value float64
}

// Histogram represents a histogram metric
type Histogram struct {
	*Metric
	buckets map[float64]uint64
}

// Registry manages metrics collection and reporting
type Registry struct {
	mu       sync.RWMutex
	metrics  map[string]*Metric
	logger   *logger.Logger
	interval time.Duration
	stopCh   chan struct{}
}

// NewRegistry creates a new metrics registry
func NewRegistry(logger *logger.Logger, interval time.Duration) *Registry {
	return &Registry{
		metrics:  make(map[string]*Metric),
		logger:   logger,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start starts the metrics collection
func (r *Registry) Start() {
	go r.collect()
}

// Stop stops the metrics collection
func (r *Registry) Stop() {
	close(r.stopCh)
}

// RegisterCounter registers a new counter metric
func (r *Registry) RegisterCounter(name, description string, labels map[string]string) *Counter {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.metrics[name]; exists {
		return nil
	}

	metric := &Metric{
		Name:        name,
		Type:        CounterType,
		Description: description,
		Labels:      labels,
		Value:       uint64(0),
		LastUpdate:  time.Now(),
	}

	counter := &Counter{
		Metric: metric,
	}

	r.metrics[name] = metric
	return counter
}

// RegisterGauge registers a new gauge metric
func (r *Registry) RegisterGauge(name, description string, labels map[string]string) *Gauge {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.metrics[name]; exists {
		return nil
	}

	metric := &Metric{
		Name:        name,
		Type:        GaugeType,
		Description: description,
		Labels:      labels,
		Value:       float64(0),
		LastUpdate:  time.Now(),
	}

	gauge := &Gauge{
		Metric: metric,
	}

	r.metrics[name] = metric
	return gauge
}

// RegisterHistogram registers a new histogram metric
func (r *Registry) RegisterHistogram(name, description string, labels map[string]string) *Histogram {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.metrics[name]; exists {
		return nil
	}

	metric := &Metric{
		Name:        name,
		Type:        HistogramType,
		Description: description,
		Labels:      labels,
		Value:       make(map[float64]uint64),
		LastUpdate:  time.Now(),
	}

	histogram := &Histogram{
		Metric:  metric,
		buckets: make(map[float64]uint64),
	}

	r.metrics[name] = metric
	return histogram
}

// GetMetric returns a metric by name
func (r *Registry) GetMetric(name string) *Metric {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.metrics[name]
}

// GetCounter returns a counter metric by name
func (r *Registry) GetCounter(name string) *Counter {
	r.mu.RLock()
	defer r.mu.RUnlock()

	metric, exists := r.metrics[name]
	if !exists || metric.Type != CounterType {
		return nil
	}
	return &Counter{
		Metric: metric,
		value:  metric.Value.(uint64),
	}
}

// GetGauge returns a gauge metric by name
func (r *Registry) GetGauge(name string) *Gauge {
	r.mu.RLock()
	defer r.mu.RUnlock()

	metric, exists := r.metrics[name]
	if !exists || metric.Type != GaugeType {
		return nil
	}
	return &Gauge{
		Metric: metric,
		value:  metric.Value.(float64),
	}
}

// GetHistogram returns a histogram metric by name
func (r *Registry) GetHistogram(name string) *Histogram {
	r.mu.RLock()
	defer r.mu.RUnlock()

	metric, exists := r.metrics[name]
	if !exists || metric.Type != HistogramType {
		return nil
	}
	return &Histogram{
		Metric:  metric,
		buckets: metric.Value.(map[float64]uint64),
	}
}

// collect collects and reports metrics
func (r *Registry) collect() {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-r.stopCh:
			return
		case <-ticker.C:
			r.mu.RLock()
			for _, metric := range r.metrics {
				fields := map[string]any{
					"name":        metric.Name,
					"type":        metric.Type.String(),
					"value":       metric.Value,
					"last_update": metric.LastUpdate,
				}

				// Add labels
				for k, v := range metric.Labels {
					fields[k] = v
				}

				r.logger.Debug("Metric collected", fields)
			}
			r.mu.RUnlock()
		}
	}
}

// Inc increments a counter
func (c *Counter) Inc() {
	atomic.AddUint64(&c.value, 1)
	c.Value = float64(c.value)
	c.LastUpdate = time.Now()
}

// Add adds a value to a counter
func (c *Counter) Add(value float64) {
	atomic.AddUint64(&c.value, uint64(value))
	c.Value = float64(c.value)
	c.LastUpdate = time.Now()
}

// Set sets a gauge value
func (g *Gauge) Set(value float64) {
	g.value = value
	g.Value = value
	g.LastUpdate = time.Now()
}

// Add adds a value to a histogram
func (h *Histogram) Add(value float64) {
	h.buckets[value]++
	h.Value = value
	h.LastUpdate = time.Now()
}

// String returns the string representation of a metric type
func (t MetricType) String() string {
	switch t {
	case CounterType:
		return "counter"
	case GaugeType:
		return "gauge"
	case HistogramType:
		return "histogram"
	default:
		return "unknown"
	}
}
