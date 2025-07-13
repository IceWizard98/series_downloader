package user

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/IceWizard98/series_downloader/models"
	bloomfilter "github.com/IceWizard98/series_downloader/utils/bloomFilter"
	"github.com/IceWizard98/series_downloader/utils/routinepool"
	"github.com/joho/godotenv"
)

var (
	instance *user
	once     sync.Once
	initErr  error
)

type user struct {
	Name    string
	RootDir string
	history []UserHistory
}

type UserHistory struct {
  Provider          string `json:"provider"`
  SeriesID          string `json:"series_id"`
	SeriesName        string `json:"series_name"`
	SeriesSlug        string `json:"series_slug"`
	SeriesTotEpisodes uint16 `json:"series_tot_episodes"`
  EpisodeID         uint   `json:"episode_id"`
	EpisodeNumber     uint16 `json:"episode_number"`
}

const (
	HISTORY_FILE = "/.history"
)

func GetInstance(name string) (*user, error) {
	once.Do(func() {
	  userHomeDir, err := os.UserHomeDir()

	  if err != nil {
	  	userHomeDir = "."
	  }

	  userHomeDir += "/.series_downloader"

	  envFile := fmt.Sprintf("%s/.%s.env", userHomeDir, name)
	  if _, err := os.Stat(envFile); err != nil {
			if err := os.MkdirAll(userHomeDir, os.ModePerm); err != nil {
				initErr = fmt.Errorf("error creating directory: %w", err)
				return
			}
			
			f, err := os.Create(envFile)
			if err != nil {
				initErr = fmt.Errorf("error creating env file: %w", err)
				return
			}
			defer f.Close()
			
			data := []string{
				"USER_ROOT_DIR=" + userHomeDir + "\n",
				"DOWNLOAD_NEXT_EPISODES=5\n",
			}

			for _, d := range data {
				if _, err := f.WriteString(d); err != nil {
					initErr = fmt.Errorf("error writing to env file: %w", err)
					return
				}
			}
	  }

		if err := godotenv.Load(envFile); err != nil {
			initErr = fmt.Errorf("error loading newly created env file: %w", err)
			return
		}

	  userRootDir := os.Getenv("USER_ROOT_DIR")

	  if userRootDir == "" {
	  	userRootDir = userHomeDir + "/.series_downloader"
	  }

	  instance = &user{
	  	Name:    name,
	  	RootDir: userRootDir,
	  }

	  bloomFilter := bloomfilter.GetInstance()
	  bloomRP     := routinepool.GetInstance().AddSubGroup("bloom", 100, 5)

	  _ = filepath.WalkDir(userRootDir, func(path string, d os.DirEntry, err error) error {
	  	if err != nil { return err }

	  	if !d.IsDir() { 
	  	  bloomRP.AddTask(func() {
	  			bloomFilter.Add([]byte(path)) 
	  	  })
	  	}

	  	return nil
	  })
	})

	return instance, initErr
}
/*
  Load from disk and return the user history
*/
func (u *user) GetHistory() []UserHistory {
	if u.history == nil {
		u.history = []UserHistory{}

	  if _, err := os.Stat(u.RootDir + HISTORY_FILE); err == nil {
	  	jsonHistory, _ := os.ReadFile(u.RootDir + HISTORY_FILE)

			_ = json.Unmarshal(jsonHistory, &u.history)
	  }
	}

	return u.history
}

/*
	Adds a new episode to the user history
*/
func (u *user) AddHistory(provider string, series models.Series, episode models.Episode) {

	if u.history == nil {
		u.GetHistory()
	}

	var history *UserHistory
	for i, h := range u.history {
		if h.Provider  != provider   { continue }
		if h.SeriesID   != series.ID   { continue }

		history = &u.history[i]
		break
	}

	if history == nil {
	  history = &UserHistory{
			Provider          : provider,
			SeriesID          : series.ID,
			SeriesName        : series.Name,
			SeriesSlug        : series.Slug,
			SeriesTotEpisodes : uint16(series.Episodes),
			EpisodeID         : episode.ID,
			EpisodeNumber     : episode.Number,
	  }
	  u.history = append(u.history, *history)
	} else {
		history.EpisodeNumber = episode.Number
		history.EpisodeID     = episode.ID
	}

	jsonHistory, _ := json.Marshal(u.history)
	err := os.WriteFile(u.RootDir + HISTORY_FILE, jsonHistory, 0664); if err != nil {
		panic(err)
	}
}
