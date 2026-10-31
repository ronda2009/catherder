package engine

import (
	"testing"

	"catherder/internal/openclaw"
)

// 正在就地修改的那条任务：算「还没完」，不能趁老板改的时候判定队列空去关机，
// 也不能被「全部异常」那条逻辑误判成停机。
func TestEditingCountsAsUnfinished(t *testing.T) {
	e := newTestEngine(t)
	id, err := e.st.AddTask("正文", "w-a", "A")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.st.StartEditing(id); err != nil {
		t.Fatal(err)
	}
	inc := map[string]bool{"w-a": true}
	idle := map[string]openclaw.Window{"w-a": win("w-a", "A", "strata", true)}
	if ok, why := e.shutdownReady(inc, idle); ok {
		t.Error("有一条正在改时不该判定可以关机")
	} else if why == "" {
		t.Error("该给出不能关机的原因")
	}
	e.checkHalt()
	if h, _ := e.st.GetSetting("halt"); h != "" {
		t.Errorf("有一条正在改不该停机等待处理，实际停着: %s", h)
	}
}
