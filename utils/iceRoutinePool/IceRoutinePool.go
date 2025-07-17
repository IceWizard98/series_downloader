package iceRoutinePool

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
)

type IceRoutinePool struct {
	Name   string
	Closed atomic.Bool

	jobs      chan func()
	wg        *sync.WaitGroup
	ctx       context.Context	
	ctxCancel context.CancelFunc	
	subGroups map[string]*IceRoutinePool
	mutex     sync.RWMutex
}

func New(name string, ctx context.Context, bufferSize uint, concurrentJobs uint) *IceRoutinePool {
	if ctx == nil {
		ctx = context.Background()
	}

	if bufferSize == 0 {
		bufferSize = 1
	}

  if concurrentJobs == 0 {
		concurrentJobs = 1
  }

	if name == "" {
		name = "main"
	}

	context, cancel := context.WithCancel(ctx)

	instance := &IceRoutinePool{
		Name      : name,
		jobs      : make(chan func(), bufferSize),
		wg        : &sync.WaitGroup{},
		ctx       : context,
		ctxCancel : cancel,
		subGroups : make(map[string]*IceRoutinePool),
		Closed    : atomic.Bool{},
	}

	for range concurrentJobs {
		go func() {
			for {
				select {
				case <-instance.ctx.Done():
					return

				case task, ok := <-instance.jobs:
					if !ok {
						return
					}

					func() {
						defer instance.wg.Done()
						task()
					}()
				}
			}
		}()
	}

	return instance
}

func (i *IceRoutinePool) isClosed() bool {
	return i.Closed.Load()
}

func (i *IceRoutinePool) AddSubGroup(name string, bufferSize uint, concurrentJobs uint) *IceRoutinePool {
	existing := i.GetSubGroup([]string{name})

	i.mutex.Lock()
	defer i.mutex.Unlock()

	if existing != nil && !existing.isClosed() {
		return existing
	}

	subGroup := New(name, i.ctx, bufferSize, concurrentJobs)
	i.subGroups[name] = subGroup
	return subGroup
}

func (i *IceRoutinePool) GetSubGroup(name []string) *IceRoutinePool {

	if len(name) == 0 {
		return i
	}

	i.mutex.RLock()
	defer i.mutex.RUnlock()

	subGroup, ok := i.subGroups[name[0]]

	if !ok {
		return nil
	}

	if len(name) > 1 {
		return subGroup.GetSubGroup(name[1:])
	}

	return subGroup
}

func (i *IceRoutinePool) AddTask(task func()) {
	if i.isClosed() { return } 
	i.wg.Add(1)
	select {
	case i.jobs <- task:
	case <-i.ctx.Done():
		i.wg.Done() 
	}
}

func (i *IceRoutinePool) Wait() {
	if i.isClosed() { return } 
	i.wg.Wait()
}

func (i *IceRoutinePool) WaitAll() {
	i.mutex.RLock()
	subGroups := make([]*IceRoutinePool, 0, len(i.subGroups))

	for _, sub := range i.subGroups {
		subGroups = append(subGroups, sub)
	}

	i.mutex.RUnlock()

	for _, sub := range subGroups {
		sub.WaitAll()
	}

	i.Wait()
}

func (i *IceRoutinePool) Close() {
	if !i.Closed.CompareAndSwap(false, true) { return } 
	close(i.jobs)
	i.Wait()
}

func (i *IceRoutinePool) CloseAll() {
	i.mutex.Lock()
	subGroups := make([]*IceRoutinePool, 0, len(i.subGroups))

	for _, sub := range i.subGroups {
		subGroups = append(subGroups, sub)
	}

	i.mutex.Unlock()

	for _, sub := range subGroups {
		sub.CloseAll()
	}

	i.Close()
}

func (i *IceRoutinePool) Cancel() {
	if i.Closed.CompareAndSwap(false, true) { return } 
	i.ctxCancel()
	close(i.jobs)
	i.Wait()
	fmt.Printf("Routine pool cancelled: %s\n", i.Name)
}

func (i *IceRoutinePool) CancelAll() {
	i.mutex.Lock()
	subGroups := make([]*IceRoutinePool, 0, len(i.subGroups))

	for name, sub := range i.subGroups {
		subGroups = append(subGroups, sub)
		delete(i.subGroups, name)
	}

	i.mutex.Unlock()

	for _, sub := range subGroups {
		sub.CancelAll()
	}

	i.Cancel()
}
