// 循环控制的死锁回归测试（用户 2026-10-25 看到的现象：顶栏徽章全是「…」、双击打不开）。
// 根因：StartLoop 已持有 a.mu，里面又调了取同一把锁的 setNextTick —— Go 互斥锁不可重入 → 永久卡住。
// 这条测试不靠界面，直接盯「拿着锁的调用里不能再取锁」这件事。
package app

import (
	"errors"
	"testing"
	"time"
)

// StartLoop / StopLoop 必须在半秒内返回；锁不可重入时它们永远不返回。
func TestStartStopLoopNoDeadlock(t *testing.T) {
	a := newTestApp(t, `{}`)

	if err := runWithin(500*time.Millisecond, a.StartLoop); err != nil {
		t.Fatalf("StartLoop 卡住（多半是锁重入死锁）: %v", err)
	}
	if !a.IsRunning() {
		t.Fatal("StartLoop 之后该在跑")
	}
	// 界面每次刷新都会调这两个：卡住时顶栏徽章会停在占位符「…」，就是用户看到的样子。
	if err := runWithin(500*time.Millisecond, func() {
		_ = a.Overview()
		_ = a.IsRunning()
	}); err != nil {
		t.Fatalf("取概览卡住: %v", err)
	}
	if err := runWithin(500*time.Millisecond, a.StopLoop); err != nil {
		t.Fatalf("StopLoop 卡住: %v", err)
	}
	if a.IsRunning() {
		t.Fatal("StopLoop 之后不该在跑")
	}
}

// 手动检查那条路也不能卡（它不持锁，走 rearmNextTick）。
func TestTickNowNoDeadlock(t *testing.T) {
	a := newTestApp(t, `{}`)
	done := make(chan string, 1)
	go func() { done <- a.TickNow() }()
	select {
	case msg := <-done:
		if msg == "" {
			t.Error("TickNow 该回一句结果文字，不能空着")
		}
	case <-time.After(20 * time.Second):
		t.Fatal("TickNow 卡住")
	}
}

func runWithin(d time.Duration, fn func()) error {
	done := make(chan struct{})
	go func() { fn(); close(done) }()
	select {
	case <-done:
		return nil
	case <-time.After(d):
		return errors.New("超过 " + d.String() + " 没返回")
	}
}
