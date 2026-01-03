package recovery

import (
	"fmt"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"drip/internal/server/metrics"
	"go.uber.org/zap"
)

type PanicMetrics struct {
	totalPanics  uint64
	recentPanics []PanicRecord
	mu           sync.Mutex
	logger       *zap.Logger
	alerter      Alerter
}

type PanicRecord struct {
	Location  string
	Timestamp time.Time
	Value     interface{}
	Stack     string
}

type Alerter interface {
	SendAlert(title string, message string)
}

func NewPanicMetrics(logger *zap.Logger, alerter Alerter) *PanicMetrics {
	return &PanicMetrics{
		recentPanics: make([]PanicRecord, 0, 100),
		logger:       logger,
		alerter:      alerter,
	}
}

func (pm *PanicMetrics) RecordPanic(location string, panicValue interface{}) {
	atomic.AddUint64(&pm.totalPanics, 1)
	metrics.PanicTotal.Inc()

	pm.mu.Lock()

	record := PanicRecord{
		Location:  location,
		Timestamp: time.Now(),
		Value:     panicValue,
		Stack:     string(debug.Stack()),
	}

	pm.recentPanics = append(pm.recentPanics, record)

	if len(pm.recentPanics) > 100 {
		pm.recentPanics = pm.recentPanics[1:]
	}

	shouldAlert := pm.shouldAlertUnlocked()
	pm.mu.Unlock()

	if shouldAlert {
		pm.sendAlert()
	}
}

func (pm *PanicMetrics) shouldAlertUnlocked() bool {
	threshold := time.Now().Add(-5 * time.Minute)
	count := 0

	for i := len(pm.recentPanics) - 1; i >= 0; i-- {
		if pm.recentPanics[i].Timestamp.After(threshold) {
			count++
		} else {
			break
		}
	}

	rate := float64(count) / 5.0
	return rate >= 2.0
}

func (pm *PanicMetrics) sendAlert() {
	total := atomic.LoadUint64(&pm.totalPanics)

	pm.mu.Lock()
	threshold := time.Now().Add(-5 * time.Minute)
	count := 0
	for i := len(pm.recentPanics) - 1; i >= 0; i-- {
		if pm.recentPanics[i].Timestamp.After(threshold) {
			count++
		} else {
			break
		}
	}
	rate := float64(count) / 5.0
	pm.mu.Unlock()

	pm.logger.Error("ALERT: High panic rate detected",
		zap.Uint64("total_panics", total),
		zap.Float64("rate_per_minute", rate),
	)

	if pm.alerter != nil {
		message := "High panic rate detected: %.2f panics/minute (total: %d)"
		pm.alerter.SendAlert(
			"Drip: High Panic Rate",
			fmt.Sprintf(message, rate, total),
		)
	}
}
