// 临时探针：把某条任务标成异常，用来看界面的「标异常的任务」区与停机横幅。
// 用法: failmark <任务id> [原因]   （只写派工台自己的库，不碰 OpenClaw、不占模型）
package main

import (
	"fmt"
	"os"

	"catherder/internal/store"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("用法: failmark <任务id> [原因]")
		os.Exit(2)
	}
	var id int
	fmt.Sscanf(os.Args[1], "%d", &id)
	note := "模型服务不通: 探针模拟（127.0.0.1:8095 连不上）"
	if len(os.Args) > 2 {
		note = os.Args[2]
	}
	st, err := store.Open(store.DefaultPath())
	if err != nil {
		fmt.Println("开库失败:", err)
		os.Exit(1)
	}
	defer st.Close()
	if err := st.MarkError(id, note); err != nil {
		fmt.Println("标异常失败:", err)
		os.Exit(1)
	}
	fmt.Printf("T-%03d 已标异常：%s\n", id, note)
}
