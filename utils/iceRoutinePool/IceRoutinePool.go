package iceRoutinePool

import (
	"context"
	"sync"
)

type IceRoutinePool struct {
	Name string
	jobs chan func()
	wg *sync.WaitGroup
	ctx context.Context	
	cancel context.CancelFunc	
	subGroups map[string]*IceRoutinePool
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
		Name: name,
		jobs: make(chan func(), bufferSize),
		wg: &sync.WaitGroup{},
		ctx: context,
		cancel: cancel,
		subGroups: make(map[string]*IceRoutinePool),
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
	subGroup := New(name, i.ctx, bufferSize, concurrentJobs)
	i.subGroups[name] = subGroup
	return subGroup
}

func (i *IceRoutinePool) GetSubGroup(name []string) *IceRoutinePool {

	if len(name) == 0 {
		return i
	}

	subGroup, ok := i.subGroups[name[0]]

	if !ok {
		return i
	}

	if len(name) > 0 {
		return subGroup.GetSubGroup(name[1:])
	}

	return subGroup
}

func (i *IceRoutinePool) AddTask(task func()) {
	i.wg.Add(1)
	i.jobs <- task
}

func (i *IceRoutinePool) Wait() {
	i.wg.Wait()
}

func (i *IceRoutinePool) WaitAll() {
	for _, sub := range i.subGroups {
		sub.WaitAll()
	}

	i.wg.Wait()
}

func (i *IceRoutinePool) Close() {
	close(i.jobs)
	i.Wait()
}

func (i *IceRoutinePool) CloseAll() {
	for _, sub := range i.subGroups {
		sub.CloseAll()
	}
	
	i.Close()
}

func (i *IceRoutinePool) Cancel() {
	close(i.jobs)
	i.Wait()
	i.cancel()
}

func (i *IceRoutinePool) CancelAll() {
	for nme, sub := range i.subGroups {
		sub.CancelAll()
		delete(i.subGroups, nme)
	}
  
	i.Cancel()
}
