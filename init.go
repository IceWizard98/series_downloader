package main

import (
	"anime_watcher/models"
	"anime_watcher/models/animeunity"
  "anime_watcher/utils/routinepoll"
	"flag"
	"strconv"
	"unicode"

	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

func main() {
	anime_title := flag.String("title", "", "Anime title")
	userName    := flag.String("user", "", "Eser.env file for configuration loading")

	flag.Parse()

	envFile := fmt.Sprintf("%s.env", *userName)

	fmt.Printf("Loading env file: %s\n", envFile)
	if _, err := os.Stat(envFile); err == nil {
		_ = godotenv.Load(envFile)
	}

  userRootDir := os.Getenv("USER_ROOT_DIR")

	if userRootDir == "" {
		panic("USER_ROOT_DIR is not set")
	}

	user := models.User{
		RootDir: userRootDir,
	}


	if *anime_title == "" || len(*anime_title) == 0 {
		fmt.Println("Please provide an anime title")
		return
	}

	animeUnityInstance := animeunity.Init()
	animeList          := animeUnityInstance.Search(*anime_title)

	if len(animeList) == 0 {
		fmt.Println("No results found")
		return
	}

	for i, v := range animeList {
		fmt.Printf("%d - %s\n", i + 1, v.Slug)
	}

	fmt.Println("Select anime")
	reader := bufio.NewReader(os.Stdin)

  selected, _ := reader.ReadString('\n') 
	selected     = strings.TrimSpace(selected)

	if selected == "" || len(selected) == 0 {
		fmt.Println("Invalid selection")
		return
	}

	for _, char := range selected {
		if !unicode.IsDigit(char) {
			fmt.Println("Only digit are allowed")
			return
		}
	}

	index_selected, err := strconv.ParseUint(selected, 10, 16); if err != nil {
		panic(err)
	}

	if index_selected < 1 || uint16(index_selected) > uint16(len(animeList)) {
		fmt.Println("Invalid selection")
		return
	}

	var selectedEpisode models.Episode
	selectedAnime := animeList[index_selected - 1]
  toContinue    := false
	for _, v := range user.GetHistory() {
		if v.AnimeID == selectedAnime.ID {
			fmt.Printf("Current episode: %d\n", v.EpisodeNumber)
			fmt.Println("Do you want to whatch the next episode? (y/n)")
			reader := bufio.NewReader(os.Stdin)

			to_continue, _ := reader.ReadString('\n') 
			to_continue    =  strings.TrimSpace(to_continue)
			to_continue    =  strings.ToLower(to_continue)

			if to_continue == "y" {
				selectedEpisode = models.Episode{
					ID: v.EpisodeID,
					Number: v.EpisodeNumber,
				}
				toContinue = true
			}
		}
	}

	var episodes []models.Episode
	if toContinue {
  	episodes = animeUnityInstance.GetEpisodes(selectedAnime)

	  if uint16(len(episodes)) <= selectedEpisode.Number {
      fmt.Println("Anime is over, well done!")
			return
		}

    fmt.Printf("Continue watching episode %d\n", selectedEpisode.Number+1)
  	selectedEpisode = episodes[selectedEpisode.Number]
	} else {
  	episodes = animeUnityInstance.GetEpisodes(selectedAnime)
  	for i, v := range episodes {
  		fmt.Printf("%d - %d\n", i + 1, v.Number)
  	}
  
    selected, _ = reader.ReadString('\n') 
  	selected    = strings.TrimSpace(selected)

		if selected == "" || len(selected) == 0 {
			fmt.Println("Invalid selection")
			return
		}

		for _, char := range selected {
			if !unicode.IsDigit(char) {
				fmt.Println("Only digit are allowed")
				return
			}
		}
  
  	index_selected, err = strconv.ParseUint(selected, 10, 16)
  	if err != nil {
  		panic(err)
  	}
  	
  	if index_selected < 1 || uint16(index_selected) > uint16(len(episodes)) {
  		fmt.Println("Invalid selection")
  		return
  	}
  
  	selectedEpisode = episodes[index_selected - 1]
	}

	pool := routinepoll.GetInstance()

	pool.AddTask( func() {
	  func( episode models.Episode ) {
	  	fmt.Printf("Downloading main episode %d\n", episode.Number)
    	path, error := animeUnityInstance.DownloadEpisode(episode, user.RootDir)

	  	if error != nil {
	  		fmt.Printf("Error downloading episode %d: %s\n", episode.Number, error)
	  	} else {
	  		fmt.Printf("Episode downloaded: %d\n", episode.Number)
	  		user.AddHistory( "animeunity", selectedAnime.ID, episode ) 
	  		fmt.Printf("Play episode: %s\n", path)
	  	}
	  }( selectedEpisode )
	})


  downloadNextNEpisodes := os.Getenv("DOWNLOAD_NEXT_EPISODES")

	if downloadNextNEpisodes == "" || len(downloadNextNEpisodes) == 0 {
		pool.Wait()
		return
	}

	for _, char := range downloadNextNEpisodes {
		if !unicode.IsDigit(char) {
			fmt.Println("Only digit are allowed in DOWNLOAD_NEXT_EPISODES")
			pool.Wait()
			return
		}
	}

	nextNEpisodes, err := strconv.ParseUint(downloadNextNEpisodes, 10, 16); if err != nil {
		fmt.Printf("Error parsing %s: %s\n", downloadNextNEpisodes, err)
	}

	fmt.Printf("Downloading %d next episodes\n", nextNEpisodes)
	// The iterator starts at 0, but the first episode has number = 1 and is at index 0 in the slice.
	// This means we can simply add the iterator to the number of episodes already downloaded —
	// the result will correctly match the index in the slice.
	for iterator := range uint16(nextNEpisodes) {
  	if uint16(len(episodes)) > selectedEpisode.Number + iterator {
			pool.AddTask( func() {
				episode := episodes[selectedEpisode.Number + iterator]
				fmt.Printf("Downloading episode %d\n", episode.Number)

				_, error := animeUnityInstance.DownloadEpisode(episode, user.RootDir)

				if error != nil {
					fmt.Printf("Error downloading episode %d: %s\n", episode.Number, error)
				} else {
					fmt.Printf("Episode downloaded: %d\n", episode.Number)
				}
			})
  	} else {
			break
		}
	}

	pool.Wait()
}
