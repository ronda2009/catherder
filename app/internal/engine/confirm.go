// 同模型并发确认：同一模型服务已有一条在执行、又要放行第二条时先请求确认。
// 界面是网页，引擎没法自己弹窗，所以这里只把「要问的那一句」挂起来，界面轮询到就弹；
// 得到答复后再放行。判定依据＝两个窗口的服务商地址（provider+端口）相同；云端无法探测，不弹。
package engine

import (
	"encoding/json"
	"fmt"
	"time"

	"catherder/internal/openclaw"
	"catherder/internal/store"
)

type ConfirmRequest struct {
	TaskID   int    `json:"taskId"`
	Target   string `json:"target"`
	Running  string `json:"running"`
	Provider string `json:"provider"`
	Endpoint string `json:"endpoint"`
	Question string `json:"question"`
}

// 两种答复凭证的有效期：放行/暂不派发均只保留 10 分钟，过期后重新按规则询问。
const confirmTTLms = 10 * 60 * 1000

// endpointOf 把服务商名换成本地端口；云端返回空。
func (e *Engine) endpointOf(provider string) string {
	if provider == "" {
		return ""
	}
	local, hostport := e.cfg.ProviderIsLocal(provider)
	if !local {
		return ""
	}
	return hostport
}

// confirmMuted 已勾选「今天不再提醒」/「本次运行不再提醒」时不再重复询问。
func (e *Engine) confirmMuted() bool {
	mode, _ := e.st.GetSetting("mute_mode")
	switch mode {
	case "run": // 本次运行：进程重启即失效（引擎不落盘，重启程序后重置）
		return true
	case "today":
		day, _ := e.st.GetSetting("mute_date")
		return day == time.Now().Format("2006-01-02")
	}
	return false
}

func (e *Engine) setConfirmPending(r ConfirmRequest) {
	b, err := json.Marshal(r)
	if err != nil {
		return
	}
	e.st.SetSetting("confirm_pending", string(b))
}

// PendingConfirm 给界面轮询：有要问的就返回那一句，没有返回 nil。
func (e *Engine) PendingConfirm() *ConfirmRequest {
	v, _ := e.st.GetSetting("confirm_pending")
	if v == "" {
		return nil
	}
	var r ConfirmRequest
	if err := json.Unmarshal([]byte(v), &r); err != nil {
		e.st.SetSetting("confirm_pending", "")
		return nil
	}
	return &r
}

// AnswerConfirm 接收界面答复：accept＝仍然派发；mute＝""|"today"|"run"。
func (e *Engine) AnswerConfirm(accept bool, mute string) error {
	r := e.PendingConfirm()
	if r == nil {
		return fmt.Errorf("没有要确认的任务了（可能已经处理过）")
	}
	e.st.SetSetting("confirm_pending", "")
	switch mute {
	case "today":
		e.st.SetSetting("mute_mode", "today")
		e.st.SetSetting("mute_date", time.Now().Format("2006-01-02"))
	case "run":
		e.st.SetSetting("mute_mode", "run")
		e.st.SetSetting("mute_date", "")
	}
	key, val := "confirm_skip", "暂不派发，保留队列"
	if accept {
		key, val = "confirm_grant", "已确认并发"
	}
	e.st.SetSetting(key, fmt.Sprintf("%d|%d", r.TaskID, time.Now().UnixMilli()+confirmTTLms))
	e.st.Log("confirm", "T-%03d %s（不再提醒=%s）", r.TaskID, val, mute)
	return nil
}

type confirmResult int

const (
	confirmPass  confirmResult = iota // 直接派
	confirmAsk                        // 已挂起确认，本次不派
	confirmBlock                      // 选择暂不派发 / 正在等待其他确认，本次不派
)

// sameModelConflict：目标窗口与执行中的某条任务使用同一本地模型服务（provider+端口相同）
// 才算冲突；云端探不到并发能力，不弹。
func (e *Engine) sameModelConflict(t store.Task, w openclaw.Window, ds []store.Task, wins map[string]openclaw.Window) *ConfirmRequest {
	ep := e.endpointOf(w.Provider)
	if ep == "" {
		return nil // 云端或探不到：不弹
	}
	for _, d := range ds {
		if d.ID == t.ID {
			continue
		}
		rw, ok := wins[d.TargetKey]
		if !ok {
			continue
		}
		if e.endpointOf(rw.Provider) == ep {
			q := fmt.Sprintf("目标窗口「%s」与正在执行的「%s」使用同一模型服务（%s）。第二条会在服务中排队，可能拖慢第一条。是否仍然派发？",
				displayName(w), displayName(rw), ep)
			return &ConfirmRequest{
				TaskID: t.ID, Target: displayName(w), Running: displayName(rw),
				Provider: w.Provider, Endpoint: ep, Question: q,
			}
		}
	}
	return nil
}

// confirmDecision 看这条任务用户表过态没有：返回「放行」「先跳过」。
func (e *Engine) confirmDecision(taskID int) (grant, skip bool) {
	now := time.Now().UnixMilli()
	if v, _ := e.st.GetSetting("confirm_grant"); v != "" {
		var id int64
		var until int64
		if n, _ := fmt.Sscanf(v, "%d|%d", &id, &until); n == 2 {
			if until < now {
				e.st.SetSetting("confirm_grant", "")
			} else if int(id) == taskID {
				e.st.SetSetting("confirm_grant", "")
				return true, false
			}
		}
	}
	if v, _ := e.st.GetSetting("confirm_skip"); v != "" {
		var id int64
		var until int64
		if n, _ := fmt.Sscanf(v, "%d|%d", &id, &until); n == 2 {
			if until < now {
				e.st.SetSetting("confirm_skip", "")
			} else if int(id) == taskID {
				return false, true
			}
		}
	}
	return false, false
}
