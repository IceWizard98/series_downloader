package iceRoutinePool

import (
	"context"
	"sync"
)

type IceRoutinePool struct {
	Name   string
	Closed bool

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
		Closed    : false,
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

					task()
					instance.wg.Done()
				}
			}
		}()
	}

	return instance
}

func (i *IceRoutinePool) AddSubGroup(name string, bufferSize uint, concurrentJobs uint) *IceRoutinePool {
	existing := i.GetSubGroup([]string{name})

	i.mutex.Lock()
	defer i.mutex.Unlock()

	if existing != nil && !existing.Closed {
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
	// if i.Closed { return } 
	i.wg.Add(1)
	i.jobs <- task
}

func (i *IceRoutinePool) Wait() {
	if i.Closed { return } 
	i.wg.Wait()
}

func (i *IceRoutinePool) WaitAll() {
	for _, sub := range i.subGroups {
		sub.WaitAll()
	}

	i.wg.Wait()
}

func (i *IceRoutinePool) Close() {
	if i.Closed { return } 
	close(i.jobs)
	i.Wait()
	i.Closed = true
}

func (i *IceRoutinePool) CloseAll() {
	for _, sub := range i.subGroups {
		sub.CloseAll()
	}
	
	if i.Closed { return } 
	i.Close()
}

func (i *IceRoutinePool) Cancel() {
	if i.Closed { return } 
	close(i.jobs)
	i.Wait()
	i.ctxCancel()
	i.Closed = true
}

func (i *IceRoutinePool) CancelAll() {
	i.mutex.Lock()
	defer i.mutex.Unlock()
	for nme, sub := range i.subGroups {
		sub.CancelAll()
		delete(i.subGroups, nme)
	}
  
	if i.Closed { return } 
	i.Cancel()
}
