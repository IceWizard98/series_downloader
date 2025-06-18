package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode"

	"github.com/IceWizard98/series_downloader/models"
	"github.com/IceWizard98/series_downloader/models/animeunity"
	"github.com/IceWizard98/series_downloader/models/user"
	bloomfilter "github.com/IceWizard98/series_downloader/utils/bloomFilter"
	"github.com/IceWizard98/series_downloader/utils/routinepoll"
	"github.com/skratchdot/open-golang/open"

	"github.com/joho/godotenv"
)

func main() {
	series_title := flag.String("title", "", "Series title")
	userName     := flag.String("user", "", "Eser.env file for configuration loading")
	delete_prev  := flag.Bool("delete", false, "Delete previus episodes")

	flag.Parse()

	envFile := fmt.Sprintf("%s.env", *userName)
	if _, err := os.Stat(envFile); err == nil {
	  fmt.Printf("Loading env file: %s\n", envFile)
		_ = godotenv.Load(envFile)
	}

	userRootDir := os.Getenv("USER_ROOT_DIR")

	if userRootDir == "" {
		userDir, err := os.UserHomeDir()

		if err != nil {
			fmt.Println("Error retriving user home directory")
			return
		}
		userRootDir = userDir + "/.series_downloader"

	}

	filter := bloomfilter.GetInstance()
	user   := user.GetInstance(*userName, userRootDir)

	fmt.Println(filter.Filter)
	if *series_title == "" || len(*series_title) == 0 {
		fmt.Println("Please provide an anime title")
		return
	}

	animeUnityInstance := animeunity.Init()
	animeList, err     := animeUnityInstance.Search(*series_title)
	if err != nil {
		fmt.Printf("Error retriving series \n%s\n", err)
		return
	}

	if len(animeList) == 0 {
		fmt.Println("No results found")
		return
	}

	for i, v := range animeList {
		fmt.Printf("%d - %s\n", i+1, v.Slug)
	}

	fmt.Println("Select anime")
	reader := bufio.NewReader(os.Stdin)

	selected, _ := reader.ReadString('\n')
	selected = strings.TrimSpace(selected)

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

	index_selected, err := strconv.ParseUint(selected, 10, 16)
	if err != nil {
		fmt.Println("Invalid selection")
		return
	}

	if index_selected < 1 || uint16(index_selected) > uint16(len(animeList)) {
		fmt.Println("Invalid selection")
		return
	}

	var selectedEpisode models.Episode
	selectedSeries := animeList[index_selected-1]
	toContinue := false
	for _, v := range user.GetHistory() {
		if v.SeriesID == selectedSeries.ID {
			fmt.Printf("Current episode: %d\n", v.EpisodeNumber)
			fmt.Println("Do you want to whatch the next episode? (y/n)")
			reader := bufio.NewReader(os.Stdin)

			to_continue, _ := reader.ReadString('\n')
			to_continue = strings.TrimSpace(to_continue)
			to_continue = strings.ToLower(to_continue)

			if to_continue == "y" {
				selectedEpisode = models.Episode{
					ID:     v.EpisodeID,
					Number: v.EpisodeNumber,
				}
				toContinue = true
			}
		}
	}

	var episodes []models.Episode
	if toContinue {
		episodes, err = animeUnityInstance.GetEpisodes(selectedSeries)
		if err != nil {
			fmt.Printf("Error retriving series \n%s\n", err)
			return
		}

		if len(episodes) == 0 {
			fmt.Println("No episodes found")
			return
		}

		if uint16(len(episodes)) <= selectedEpisode.Number {
			fmt.Println("Series is over, well done!")
			return
		}

		fmt.Printf("Continue watching episode %d\n", selectedEpisode.Number+1)
		selectedEpisode = episodes[selectedEpisode.Number]
	} else {
		episodes, err = animeUnityInstance.GetEpisodes(selectedSeries)
		if err != nil {
			fmt.Printf("Error retriving series \n%s\n", err)
			return
		}

		if len(episodes) == 0 {
			fmt.Println("No episodes found")
			return
		}

		for i, v := range episodes {
			fmt.Printf("%d - %d\n", i+1, v.Number)
		}

		selected, _ = reader.ReadString('\n')
		selected = strings.TrimSpace(selected)

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

		selectedEpisode = episodes[index_selected-1]
	}

	pool := routinepoll.GetInstance()

	pool.AddTask(func() {
		func(episode models.Episode) {
			path, error := animeUnityInstance.DownloadEpisode(episode, user.RootDir)

			if error != nil {
				fmt.Printf("Error downloading episode %d: \n%s\n", episode.Number, error)
				return
			}

			fmt.Printf("Episode downloaded: %d\n", episode.Number)
			stat, err := os.Stat(path)
			if err != nil || stat.Size() <= 0 || stat.IsDir() {
				fmt.Printf("Error reading file to Play episode %s: \nerror %s\nsize %d\n", path, err, stat.Size())
				return
			}

			if err := open.Run(path); err != nil {
				fmt.Printf("Error opening file to Play episode %s: \n%s\n", path, err)
			}
			user.AddHistory("animeunity", selectedSeries, episode)
		}(selectedEpisode)
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

	nextNEpisodes, err := strconv.ParseUint(downloadNextNEpisodes, 10, 16)
	if err != nil {
		fmt.Printf("Error parsing %s: %s\n", downloadNextNEpisodes, err)
	}

	// The iterator starts at 0, but the first episode has number = 1 and is at index 0 in the slice.
	// This means we can simply add the iterator to the number of episodes already downloaded —
	// the result will correctly match the index in the slice.
	for iterator := range uint16(nextNEpisodes) {
		if uint16(len(episodes)) > selectedEpisode.Number+iterator {
			pool.AddTask(func() {
				episode := episodes[selectedEpisode.Number+iterator]
				// fmt.Printf("Downloading episode %d\n", episode.Number)

				_, error := animeUnityInstance.DownloadEpisode(episode, user.RootDir)

				if error != nil {
					fmt.Printf("Error downloading episode %d: \n%s\n", episode.Number, error)
					return
				}

				fmt.Printf("Episode downloaded: %d\n", episode.Number)
			})
		} else {
			break
		}
	}

	if *delete_prev {
		basePath := fmt.Sprintf(user.RootDir+"/%s", selectedSeries.Slug)
		files, err := os.ReadDir(basePath)
		if err != nil {
			fmt.Printf("Error reading directory %s: \n%s\n", basePath, err)
		}

		for _, file := range files {
			pool.AddTask(func() {
				func(file os.DirEntry) {
					episodeNumber, err := strconv.Atoi(strings.Split(file.Name(), ".")[0])
					if err != nil {
						fmt.Printf("Error parsing file name %s: \n%s\n", file.Name(), err)
						return
					}

					if episodeNumber >= int(selectedEpisode.Number) {
						return
					}

					fmt.Printf("Deleting file %s\n", basePath+"/"+file.Name())

					if err := os.Remove(basePath + "/" + file.Name()); err != nil {
						fmt.Printf("Error deleting file %s: \n%s\n", basePath+"/"+file.Name(), err)
					}
				}(file)
			})
		}
	}

	pool.Wait()
	// TODO: currently useless but filter must be updated on --serve version
}
