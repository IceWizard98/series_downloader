package routinepoll

import (
	"fmt"
	"os"
	"strconv"
	"sync"
	"unicode"
)

var istance *routinePool
const MAX_CONCURRENT_DOWNLOADS = "5"
type routinePool struct {
	jobs chan func()
  wg *sync.WaitGroup
}

func GetInstance() *routinePool {
	if istance == nil {
		maxConcurrentDownloads := os.Getenv("MAX_CONCURRENT_DOWNLOADS")

		if maxConcurrentDownloads == "" || len(maxConcurrentDownloads) == 0 {
			maxConcurrentDownloads = MAX_CONCURRENT_DOWNLOADS
		}

		for _, char := range maxConcurrentDownloads {
			if !unicode.IsDigit(char) {
				fmt.Println("Only digit are allowed in MAX_CONCURRENT_DOWNLOADS")
				maxConcurrentDownloads = MAX_CONCURRENT_DOWNLOADS
				break
			}
		}

		poolSize, err := strconv.ParseUint(maxConcurrentDownloads, 10, 16)
		if err != nil {
			panic(err)
		}

		istance = &routinePool{
			jobs: make(chan func(), poolSize),
			wg: &sync.WaitGroup{},
		}

		for range poolSize {
			go func() {
				for task := range istance.jobs {
					task()
					istance.wg.Done()
				}
			}()
		}
	}
	return istance
}

func (r *routinePool) AddTask(task func()) {
	r.wg.Add(1)
	r.jobs <- task
}

func (r *routinePool) Wait() {
	r.wg.Wait()
}

