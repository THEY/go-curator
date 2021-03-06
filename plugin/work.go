package plugin

import (
	. "github.com/talbright/go-curator"
	"github.com/talbright/go-zookeeper/zk"

	"sync"
)

type Work struct {
	Znode
	Children *ChildCache
}

func (w Work) Id() string { return w.Path }

func NewWork(client *Client, path string) *Work {
	n := NewZnode(path)
	cc := NewChildCache(client, path)
	cc.CreateFlags = zk.FlagEphemeral
	return &Work{Znode: *n, Children: cc}
}

func (w Work) Spew() string {
	return SpewableWrapper(nil, w)
}

type Worker struct {
	Znode
	Children *ChildCache
}

func (w Worker) Id() string { return w.Path }

func NewWorker(client *Client, path string) *Worker {
	n := NewZnode(path)
	cc := NewChildCache(client, path)
	cc.CreateFlags = zk.FlagEphemeral
	return &Worker{Znode: *n, Children: cc}
}

func (w *Worker) ShiftWork(amount int) []Znode {
	removed := make([]Znode, 0)
	size := w.Children.Size()
	if size >= amount && amount > 0 {
		for _, v := range w.Children.ToSlice()[0:amount] {
			newNode := v
			if err := w.Children.Remove(&newNode); err != nil {
				//TODO: log.WithError(err).Warn("unable to remove worker")
			}
			removed = append(removed, newNode)
		}
	}
	return removed
}

func (w *Worker) UnshiftWork(nodes []Znode) {
	for _, n := range nodes {
		newNode := n
		if err := w.Children.Add(&newNode); err != nil {
			//TODO: log.WithError(err).Warn("unable to add node")
		}
	}
}

func (w Worker) Spew() string {
	return SpewableWrapper(nil, w)
}

type WorkerList struct {
	mutex   *sync.Mutex
	workers []*Worker
}

func NewWorkerList() *WorkerList { return &WorkerList{mutex: &sync.Mutex{}} }

func (l *WorkerList) Add(worker *Worker) (added bool) {
	if exists := l.IndexOf(worker); exists == nil {
		l.mutex.Lock()
		defer l.mutex.Unlock()
		l.workers = append(l.workers, worker)
		added = true
	}
	return added
}

func (l *WorkerList) Remove(worker *Worker) (removed bool) {
	if exists := l.IndexOf(worker); exists != nil {
		l.mutex.Lock()
		defer l.mutex.Unlock()
		l.workers = append(l.workers[:(*exists)], l.workers[(*exists)+1:]...)
		removed = true
	}
	return removed
}

func (l *WorkerList) IndexOf(worker *Worker) (index *int) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	for i, v := range l.workers {
		if v.Id() == worker.Id() {
			index = new(int)
			*index = i
		}
	}
	return index
}

func (l *WorkerList) Size() int {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	return len(l.workers)
}

func (l *WorkerList) At(index int) (w *Worker) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if len(l.workers) > index {
		w = l.workers[index]
	}
	return w
}

func (l *WorkerList) FindById(id string) (index int, w *Worker) {
	return l.Find(func(i int, w *Worker) bool {
		if w.Id() == id {
			return true
		}
		return false
	})
}

func (l *WorkerList) Find(f func(int, *Worker) bool) (index int, w *Worker) {
	l.mutex.Lock()
	workers := make([]*Worker, 0)
	for _, v := range l.workers {
		workers = append(workers, v)
	}
	l.mutex.Unlock()
	for i, v := range workers {
		if f(i, v) {
			return i, v
		}
	}
	return -1, nil
}

func (l *WorkerList) ToSlice() []Worker {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	workers := make([]Worker, 0)
	for _, v := range l.workers {
		workers = append(workers, *v)
	}
	return workers
}

func (w WorkerList) Spew() string {
	return SpewableWrapper(nil, w)
}
