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

func searchForSeries(animeUnityInstance *animeunity.AnimeUnity, title string) (models.Series, error) {
	if title == "" || len(title) == 0 {
		return models.Series{}, fmt.Errorf("Please provide sires title with --title flag")
	}

	seriesList, err := animeUnityInstance.Search(title)
	if err != nil {
		return models.Series{}, fmt.Errorf("⚠️ Error retriving series \n\t- %s\n", err)
	}

	if len(seriesList) == 0 {
		return models.Series{}, fmt.Errorf("No results found")
	}

	for i, v := range seriesList {
		fmt.Printf("%d - %s\n", i+1, v.Slug)
	}

	fmt.Println("Select a series")
	reader := bufio.NewReader(os.Stdin)

	selected, _ := reader.ReadString('\n')
	selected = strings.TrimSpace(selected)

	if selected == "" || len(selected) == 0 {
		return models.Series{}, fmt.Errorf("Invalid selection")
	}

	for _, char := range selected {
		if !unicode.IsDigit(char) {
			return models.Series{}, fmt.Errorf("Only digit are allowed")
		}
	}

	index_selected, err := strconv.ParseUint(selected, 10, 16)
	if err != nil {
		return models.Series{}, fmt.Errorf("Invalid selection")
	}

	if index_selected < 1 || uint16(index_selected) > uint16(len(seriesList)) {
		return models.Series{}, fmt.Errorf("Invalid selection")
	}

	return seriesList[index_selected-1], nil
}

func main() {
	series_title := flag.String("title", "", "Series title")
	userName     := flag.String("user", "", "User.env file for configuration loading")
	delete_prev  := flag.Bool("delete", false, "Delete previus episodes")
	list         := flag.Bool("list", false, "Show list of following series")

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
			fmt.Println("⚠️ Error retriving user home directory")
			os.Exit(1)
		}
		userRootDir = userDir + "/.series_downloader"

	}

	bloomfilter.GetInstance()

	user := user.GetInstance(*userName, userRootDir)

	var selectedSeries models.Series
	animeUnityInstance := animeunity.Init()

	if *list {
		watchingSeries := user.GetHistory()
		for i, h := range watchingSeries {
			fmt.Printf("%d) %s - %s: %d\n", i+1, h.SeriesName, h.SeriesSlug, h.EpisodeNumber)
		}

		fmt.Println("Select a series")
		reader := bufio.NewReader(os.Stdin)

		selected, _ := reader.ReadString('\n')
		selected = strings.TrimSpace(selected)

		if selected == "" || len(selected) == 0 {
			fmt.Printf("⚠️ Invalid selection\n")
			os.Exit(1)
		}

		for _, char := range selected {
			if !unicode.IsDigit(char) {
				fmt.Printf("⚠️ Only digit are allowed\n")
				os.Exit(1)
			}
		}

		index_selected, err := strconv.ParseUint(selected, 10, 16)
		if err != nil {
			fmt.Printf("⚠️ Invalid selection\n")
			os.Exit(1)
		}

		if index_selected < 1 || uint16(index_selected) > uint16(len(watchingSeries)) {
			fmt.Printf("⚠️ Invalid selection\n")
			os.Exit(1)
		}

		toWatch := watchingSeries[index_selected-1]
		selectedSeries = models.Series{
      ID       : toWatch.SeriesID,
      Name     : toWatch.SeriesName,
      Slug     : toWatch.SeriesSlug,
      Episodes : uint(toWatch.SeriesTotEpisodes),
		}
	} else {
		var err error

		if selectedSeries, err = searchForSeries(animeUnityInstance, *series_title); err != nil {
			fmt.Printf("⚠️ Error retriving series \n\t- %s\n", err)
			os.Exit(1)
		}
	}

	var selectedEpisode models.Episode
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
		fmt.Printf("Continue watching episode %d\n", selectedEpisode.Number+1)
		selectedEpisode = models.Episode{
			Number: selectedEpisode.Number + 1,
		}

		var err error
		episodes, err = animeUnityInstance.GetEpisodes(selectedSeries)

		if err != nil {
			fmt.Printf("⚠️ Error retriving series \n\t- %s\n", err)
			fmt.Println("Continue to watch locally")
		}
	} else {
		episodes, err := animeUnityInstance.GetEpisodes(selectedSeries)
		if err != nil {
			fmt.Printf("⚠️ Error retriving series \n\t- %s\n", err)
			os.Exit(1)
		}

		if len(episodes) == 0 {
			fmt.Println("No episodes found")
			os.Exit(1)
		}

		for i, v := range episodes {
			fmt.Printf("%d - %d\n", i+1, v.Number)
		}

		reader      := bufio.NewReader(os.Stdin)
		selected, _ := reader.ReadString('\n')
		selected     = strings.TrimSpace(selected)

		if selected == "" || len(selected) == 0 {
			fmt.Println("Invalid selection")
			os.Exit(1)
		}

		for _, char := range selected {
			if !unicode.IsDigit(char) {
				fmt.Println("Only digit are allowed")
				os.Exit(1)
			}
		}

		index_selected, err := strconv.ParseUint(selected, 10, 16)
		if err != nil {
			fmt.Println("Invalid selection")
			os.Exit(1)
		}

		if index_selected < 1 || uint16(index_selected) > uint16(len(episodes)) {
			fmt.Println("Invalid selection")
			os.Exit(1)
		}

		selectedEpisode = episodes[index_selected-1]
	}

	pool := routinepoll.GetInstance()

	pool.AddTask(func() {
		func(episode models.Episode) {
			path, error := animeUnityInstance.DownloadEpisode(episode, user.RootDir)

			fmt.Printf("⬇️ Downloading episode %d\n", episode.Number)
			if error != nil {
				fmt.Printf("⚠️ Error downloading episode %d: \n\t- %s\n", episode.Number, error)
				return
			}

			fmt.Printf("✅ Episode downloaded: %d\n", episode.Number)
			stat, err := os.Stat(path)
			if err != nil || stat.Size() <= 0 || stat.IsDir() {
				fmt.Printf("⚠️ Error reading file to Play episode %s: \nerror %s\nsize %d\n", path, err, stat.Size())
				return
			}

			if err := open.Run(path); err != nil {
				fmt.Printf("⚠️ Error opening file to Play episode %s: \n\t- %s\n", path, err)
				return
			}
			user.AddHistory("animeunity", selectedSeries, episode)
		}(selectedEpisode)
	})

	downloadNextNEpisodes := os.Getenv("DOWNLOAD_NEXT_EPISODES")

	for _, char := range downloadNextNEpisodes {
		if !unicode.IsDigit(char) {
			fmt.Println("Only digit are allowed in DOWNLOAD_NEXT_EPISODES")
		}
	}

	nextNEpisodes, err := strconv.ParseUint(downloadNextNEpisodes, 10, 16)
	if err != nil {
		fmt.Printf("⚠️ Error parsing %s: %s\n", downloadNextNEpisodes, err)
	}

	// The iterator starts at 0, but the first episode has number = 1 and is at index 0 in the slice.
	// This means we can simply add the iterator to the number of episodes already downloaded —
	// the result will correctly match the index in the slice.
	for iterator := range uint16(nextNEpisodes) {
		if uint16(len(episodes)) > selectedEpisode.Number+iterator {
			pool.AddTask(func() {
				episode := episodes[selectedEpisode.Number+iterator]
				fmt.Printf("⬇️ Downloading episode %d\n", episode.Number)

				_, error := animeUnityInstance.DownloadEpisode(episode, user.RootDir)

				if error != nil {
					fmt.Printf("⚠️ Error downloading episode %d: \n\t- %s\n", episode.Number, error)
					return
				}

				fmt.Printf("✅ Episode downloaded: %d\n", episode.Number)
			})
		} else {
			break
		}
	}

	if *delete_prev {
		basePath := fmt.Sprintf("%s/%s", user.RootDir, selectedSeries.Slug)
		files, err := os.ReadDir(basePath)
		if err != nil {
			fmt.Printf("⚠️ Error reading directory to delete %s: \n\t- %s\n", basePath, err)
		} else 
		{
		  for _, file := range files {
		  	pool.AddTask(func() {
		  		func(file os.DirEntry) {
		  			episodeNumber, err := strconv.Atoi(strings.Split(file.Name(), ".")[0])
		  			if err != nil {
		  				fmt.Printf("⚠️ Error parsing file name to delete %s: \n\t- %s\n", file.Name(), err)
		  				return
		  			}

		  			if episodeNumber >= int(selectedEpisode.Number) {
		  				return
		  			}

		  			fmt.Printf("❌ Deleting file %s\n", basePath+"/"+file.Name())

		  			if err := os.Remove(basePath + "/" + file.Name()); err != nil {
		  				fmt.Printf("⚠️ Error deleting file %s: \n\t- %s\n", basePath+"/"+file.Name(), err)
		  			}
		  		}(file)
		  	})
		  }
		}

	}

	pool.Wait()
	// TODO: currently useless but filter must be updated on --serve version
}
