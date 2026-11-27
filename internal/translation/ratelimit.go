package translation

import (
	"context"
	"sync"
	"time"
)

type charSample struct {
	at    time.Time
	chars int
}

type engineQuotaState struct {
	mu      sync.Mutex
	nextAt  time.Time
	samples []charSample
}

type rateLimiter struct {
	mu     sync.Mutex
	states map[EngineID]*engineQuotaState
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{states: make(map[EngineID]*engineQuotaState)}
}

func (l *rateLimiter) state(id EngineID) *engineQuotaState {
	l.mu.Lock()
	defer l.mu.Unlock()
	st, ok := l.states[id]
	if !ok {
		st = &engineQuotaState{}
		l.states[id] = st
	}
	return st
}

// Wait 根据引擎的 QPS 与每分钟字符窗口（CPM）进行节流，支持 ctx 取消。
func (l *rateLimiter) Wait(ctx context.Context, id EngineID, caps EngineCapabilities, chars int, background bool) error {
	if l == nil {
		return nil
	}
	qps := caps.DefaultQPS
	cpm := caps.CharsPerMinute
	if background {
		if caps.BackgroundQPS > 0 {
			qps = caps.BackgroundQPS
		}
		if caps.BackgroundCPM > 0 {
			cpm = caps.BackgroundCPM
		}
	}
	if qps <= 0 && cpm <= 0 {
		return nil
	}
	st := l.state(id)
	for {
		st.mu.Lock()
		if err := ctx.Err(); err != nil {
			st.mu.Unlock()
			return err
		}
		now := time.Now()
		var wait time.Duration

		if qps > 0 {
			if now.Before(st.nextAt) {
				wait = st.nextAt.Sub(now)
			}
		}

		if cpm > 0 && chars > 0 {
			cutoff := now.Add(-time.Minute)
			kept := st.samples[:0]
			used := 0
			for _, s := range st.samples {
				if s.at.After(cutoff) {
					kept = append(kept, s)
					used += s.chars
				}
			}
			st.samples = kept
			if used > 0 && used+chars > cpm && len(st.samples) > 0 {
				// 等待最早的窗口样本过期
				cpmWait := st.samples[0].at.Add(time.Minute).Sub(now) + 50*time.Millisecond
				if cpmWait > wait {
					wait = cpmWait
				}
			}
		}

		if wait <= 0 {
			if qps > 0 {
				interval := time.Duration(float64(time.Second) / qps)
				st.nextAt = now.Add(interval)
			}
			if cpm > 0 && chars > 0 {
				st.samples = append(st.samples, charSample{at: now, chars: chars})
			}
			st.mu.Unlock()
			return nil
		}
		st.mu.Unlock()

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
