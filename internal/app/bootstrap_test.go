package app

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"
)

func newTestService(t *testing.T, interval time.Duration) (*BootstrapService, <-chan OperationEvent) {
	t.Helper()
	events := make(chan OperationEvent, 128)
	s := newBootstrapService(context.Background(), func(event OperationEvent) { events <- event }, interval)
	t.Cleanup(s.close)
	return s, events
}

func terminalEvent(t *testing.T, events <-chan OperationEvent) OperationEvent {
	t.Helper()
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	last := 1
	for {
		select {
		case event := <-events:
			if event.Sequence != last+1 || event.Sequence != event.Payload.Sequence {
				t.Fatalf("事件顺序不连续：last=%d event=%+v", last, event)
			}
			last = event.Sequence
			if event.Type == "operation.terminal" {
				return event
			}
		case <-timer.C:
			t.Fatal("诊断未在上限内结束")
		}
	}
}

func TestProbeCompletesAndSnapshotMatchesEvents(t *testing.T) {
	s, events := newTestService(t, time.Millisecond)
	start := s.StartProbe()
	if !start.OK || start.Data.Status != "running" || start.Data.Sequence != 1 {
		t.Fatalf("开始结果错误：%+v", start)
	}
	event := terminalEvent(t, events)
	if event.Payload.Status != "succeeded" || event.Payload.Completed != event.Payload.Total {
		t.Fatalf("未正常完成：%+v", event)
	}
	got := s.GetOperation(OperationRequest{start.Data.OperationID})
	if !got.OK || *got.Data != event.Payload {
		t.Fatalf("快照与终态不一致：%+v", got)
	}
	if cancelled := s.Cancel(OperationRequest{start.Data.OperationID}); cancelled.Data.Status != "succeeded" {
		t.Fatal("取消已完成任务不应改变终态")
	}
}

func TestConcurrentStartsCoalesceAndCancellationIsIdempotent(t *testing.T) {
	s, events := newTestService(t, time.Hour)
	ids := make(chan string, 32)
	var wg sync.WaitGroup
	for range 32 {
		wg.Go(func() { ids <- s.StartProbe().Data.OperationID })
	}
	wg.Wait()
	close(ids)
	var id string
	for current := range ids {
		if id != "" && current != id {
			t.Fatal("并发开始创建了多个活动任务")
		}
		id = current
	}
	for range 5 {
		if result := s.Cancel(OperationRequest{id}); !result.OK {
			t.Fatal("重复取消应幂等")
		}
	}
	event := terminalEvent(t, events)
	if event.Payload.Status != "cancelled" || event.Payload.Completed != 0 {
		t.Fatalf("取消仍产生进度：%+v", event)
	}
	select {
	case extra := <-events:
		t.Fatalf("终态之后有多余事件：%+v", extra)
	default:
	}
}

func TestShutdownCancelsWorkAndRejectsNewOperations(t *testing.T) {
	s, events := newTestService(t, time.Hour)
	start := s.StartProbe()
	s.close()
	if event := terminalEvent(t, events); event.Payload.Status != "cancelled" {
		t.Fatal("关闭时未取消活动任务")
	}
	if got := s.GetOperation(OperationRequest{start.Data.OperationID}); got.Data.Status != "cancelled" {
		t.Fatal("关闭后的快照不正确")
	}
	if got := s.StartProbe(); got.OK || got.Error.Code != "Unavailable" {
		t.Fatal("关闭后仍可开始任务")
	}
}

func TestErrorEnvelopeAndInputValidation(t *testing.T) {
	s, _ := newTestService(t, time.Hour)
	for _, test := range []struct{ id, code string }{{"", "ValidationError"}, {"unknown", "NotFound"}} {
		for _, result := range []OperationResult{s.GetOperation(OperationRequest{test.id}), s.Cancel(OperationRequest{test.id})} {
			if result.OK || result.Data != nil || result.Error.Code != test.code || result.RequestID == "" {
				t.Fatalf("错误载荷不正确：%+v", result)
			}
			encoded, err := json.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}
			var decoded map[string]any
			if err := json.Unmarshal(encoded, &decoded); err != nil {
				t.Fatal(err)
			}
			if _, exists := decoded["data"]; exists {
				t.Fatal("失败结果不应包含 data")
			}
		}
	}
}

func TestRecentHistoryIsBounded(t *testing.T) {
	s, events := newTestService(t, time.Hour)
	var first string
	for i := range historyLimit + 1 {
		start := s.StartProbe()
		if i == 0 {
			first = start.Data.OperationID
		}
		s.Cancel(OperationRequest{start.Data.OperationID})
		terminalEvent(t, events)
	}
	if got := s.GetOperation(OperationRequest{first}); got.OK || got.Error.Code != "NotFound" {
		t.Fatal("最旧历史未被回收")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.jobs) != historyLimit {
		t.Fatalf("历史记录无界：%d", len(s.jobs))
	}
}
