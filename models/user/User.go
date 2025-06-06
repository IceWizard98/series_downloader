package user

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/IceWizard98/series_downloader/models"
	bloomfilter "github.com/IceWizard98/series_downloader/utils/bloomFilter"
)

var instance *user

type user struct {
	Name    string
	RootDir string
	history []userHistory
}

type userHistory struct {
  Provider      string `json:"provider"`
  AnimeID       string  `json:"anime_id"`
  EpisodeID     uint   `json:"episode_id"`
	EpisodeNumber uint16 `json:"episode_number"`
}

const (
	HISTORY_FILE = "/history.json"
)

func GetInstance(name string, rootDir string) *user {
	if instance == nil {
		instance = &user{
			Name:    name,
			RootDir: rootDir,
		}
	}

	bloomFilter := bloomfilter.GetInstance()
  filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
		if err != nil { return err }

		if !d.IsDir() { bloomFilter.Add([]byte(path)) }

		return nil
	})

	return instance
}
/*
  Load from disk and return the user history
*/
func (u *user) GetHistory() []userHistory {
	if u.history == nil {
		u.history = []userHistory{}

	  if _, err := os.Stat(u.RootDir + HISTORY_FILE); err == nil {
	  	jsonHistory, _ := os.ReadFile(u.RootDir + HISTORY_FILE)

	  	json.Unmarshal(jsonHistory, &u.history)
	  }
	}

	return u.history
}

/*
	Adds a new episode to the user history
*/
func (u *user) AddHistory(provider string, animeID string, episode models.Episode) {

	if u.history == nil {
		u.GetHistory()
	}

	var history *userHistory
	for i, h := range u.history {
		if h.Provider  != provider   { continue }
		if h.AnimeID   != animeID    { continue }
		if h.EpisodeID == episode.ID { return }

		history = &u.history[i]
		break
	}

	if history == nil {
	  history = &userHistory{
	  	Provider      : provider,
	  	AnimeID       : animeID,
	  	EpisodeID     : episode.ID,
	  	EpisodeNumber : episode.Number,
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
