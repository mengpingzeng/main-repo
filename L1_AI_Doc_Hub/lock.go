package a4md

import "sync"

// taskLockSet 按 task_id 管理互斥锁
// 保证同一 task 的追加写串行执行
type taskLockSet struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

func newTaskLockSet() *taskLockSet {
	return &taskLockSet{locks: make(map[string]*sync.Mutex)}
}

// acquire 获取指定 task 的锁，返回解锁函数
func (s *taskLockSet) acquire(taskID string) func() {
	s.mu.Lock()
	mu, ok := s.locks[taskID]
	if !ok {
		mu = &sync.Mutex{}
		s.locks[taskID] = mu
	}
	s.mu.Unlock()

	mu.Lock()
	return func() { mu.Unlock() }
}
