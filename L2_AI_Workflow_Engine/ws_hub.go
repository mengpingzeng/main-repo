package workflow_engine

import (
	"encoding/json"
	"fmt"
	"sync"
)

type Client struct {
	Ch     chan []byte
	TaskID string
}

func NewClient(taskID string) *Client {
	return &Client{
		Ch:     make(chan []byte, 32),
		TaskID: taskID,
	}
}

type WSHub struct {
	mu      sync.RWMutex
	clients map[string]map[*Client]bool
}

func NewWSHub() *WSHub {
	return &WSHub{
		clients: make(map[string]map[*Client]bool),
	}
}

func (h *WSHub) Subscribe(taskID string) *Client {
	c := NewClient(taskID)
	h.mu.Lock()
	if h.clients[taskID] == nil {
		h.clients[taskID] = make(map[*Client]bool)
	}
	h.clients[taskID][c] = true
	h.mu.Unlock()
	return c
}

func (h *WSHub) Unsubscribe(c *Client) {
	h.mu.Lock()
	if clients, ok := h.clients[c.TaskID]; ok {
		delete(clients, c)
	}
	close(c.Ch)
	h.mu.Unlock()
}

func (h *WSHub) Broadcast(taskID string, event WSEvent) {
	data, _ := json.Marshal(event)
	data = append(data, '\n')

	h.mu.RLock()
	defer h.mu.RUnlock()

	for c := range h.clients[taskID] {
		select {
		case c.Ch <- data:
		default:
		}
	}
}

func (e *Engine) pushWS(task *WorkflowTask, stage, status string, errMsg string) {
	if e.wsHub == nil {
		return
	}
	progress := buildProgress(task)
	e.wsHub.Broadcast(task.TaskID, WSEvent{
		TaskID:   task.TaskID,
		Stage:    stage,
		Status:   status,
		Progress: progress,
		Err:      errMsg,
	})
}

func buildProgress(task *WorkflowTask) string {
	switch task.Status {
	case StatusFetchDraft:
		return fmt.Sprintf("正在读取第 %d 版草稿...（第 %d 次尝试）", task.DraftVersion, task.StepRetry)
	case StatusPublishing:
		return fmt.Sprintf("正在发布到 %s...", task.Platform)
	case StatusPublished:
		return buildPublishSummary(task.PublishResults)
	case StatusMDWriting:
		return "正在生成任务档案..."
	case StatusDone:
		return "全部发布完成"
	case StatusDonePartial:
		return buildPublishSummary(task.PublishResults)
	default:
		return task.Status
	}
}

func buildPublishSummary(results []PublishResult) string {
	ok, fail := 0, 0
	for _, r := range results {
		if r.Status == "ok" {
			ok++
		} else {
			fail++
		}
	}
	return fmt.Sprintf("%d 个账号已发布（%d 成功, %d 失败）", len(results), ok, fail)
}
