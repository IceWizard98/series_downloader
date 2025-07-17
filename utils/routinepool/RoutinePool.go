package routinepool

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"unicode"

	"github.com/IceWizard98/series_downloader/utils/iceRoutinePool"
)

var (
	instance *iceRoutinePool.IceRoutinePool
	once     sync.Once
	Cancel   context.CancelFunc
)
const MAX_CONCURRENT_DOWNLOADS = "5"

func GetInstance() *iceRoutinePool.IceRoutinePool {
	once.Do(func() {
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

		ctx, cancel := context.WithCancel(context.Background())
		Cancel   = cancel
		instance = iceRoutinePool.New( "main", ctx, uint(poolSize), uint(poolSize) )
	})
	return instance
}

