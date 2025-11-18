package ws

import (
	"sync"
)

// Deque is a thread-safe double-ended queue for Tasks.
type Deque struct {
	mu    sync.Mutex
	tasks []Task
}

// NewDeque creates an empty deque.
func NewDeque() *Deque {
	return &Deque{}
}

// PushBottom adds a task to the bottom (tail) of the deque.
// Used by a worker for its own tasks.
func (d *Deque) PushBottom(task Task) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.tasks = append(d.tasks, task)
}

// PopBottom removes and returns a task from the bottom (tail) of the deque.
// Used by a worker for its own tasks. Returns false if deque is empty.
func (d *Deque) PopBottom() (Task, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.tasks) == 0 {
		return Task{}, false
	}
	task := d.tasks[len(d.tasks)-1]
	d.tasks = d.tasks[:len(d.tasks)-1]
	return task, true
}

// PopTop removes and returns a task from the top (head) of the deque.
// Used by a worker to steal tasks from another worker's deque. Returns false if deque is empty.
func (d *Deque) PopTop() (Task, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.tasks) == 0 {
		return Task{}, false
	}
	task := d.tasks[0]
	d.tasks = d.tasks[1:]
	return task, true
}

// IsEmpty returns true if the deque contains no tasks.
func (d *Deque) IsEmpty() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.tasks) == 0
}

// Size returns the number of tasks in the deque.
func (d *Deque) Size() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.tasks)
}
