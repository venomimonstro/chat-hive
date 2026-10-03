package httpserver

import "sync/atomic"

type PressureGate struct {
	high     int32
	low      int32
	degraded atomic.Bool
}

func NewPressureGate(highPercent, lowPercent int32) *PressureGate {
	if highPercent <= 0 || highPercent > 100 {
		highPercent = 85
	}
	if lowPercent < 0 || lowPercent >= highPercent {
		lowPercent = 70
	}
	return &PressureGate{high: highPercent, low: lowPercent}
}

func (g *PressureGate) Update(acquired, max int32) bool {
	if max <= 0 {
		g.degraded.Store(false)
		return false
	}
	percent := acquired * 100 / max
	if g.degraded.Load() {
		if percent <= g.low {
			g.degraded.Store(false)
		}
	} else if percent >= g.high {
		g.degraded.Store(true)
	}
	return g.degraded.Load()
}

func (g *PressureGate) Degraded() bool { return g.degraded.Load() }
