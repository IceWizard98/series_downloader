package cmd

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"unicode"

	"github.com/IceWizard98/series_downloader/models"
	"github.com/IceWizard98/series_downloader/models/animeunity"
	"github.com/IceWizard98/series_downloader/models/user"
	"github.com/IceWizard98/series_downloader/utils/routinepool"
	"github.com/skratchdot/open-golang/open"
	"github.com/spf13/cobra"
)

var(
	series_title string
	userName     string
	delete_prev  bool
	list         bool
)

var rootCmd = &cobra.Command{
	Use   : "Series Downloader",
	Short : "Series Downloader",
	Long  : "Download series from different providers",
	Run   : func(cmd *cobra.Command, args []string) {
		user, err := user.GetInstance(userName)

		if err != nil {
			fmt.Printf("⚠️ %s\n", err)
			os.Exit(1)
		}

		var selectedSeries models.Series
		animeUnityInstance, err := animeunity.Init()

		if err != nil {
			fmt.Printf("⚠️ %s\n", err)
		}

		if list {
			watchingSeries := user.GetHistory()

			if len(watchingSeries) == 0 {
				fmt.Println("You are not watching any series")
				os.Exit(1)
			}

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

			if selectedSeries, err = searchForSeries(animeUnityInstance, series_title); err != nil {
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

		downloadNextNEpisodes := os.Getenv("DOWNLOAD_NEXT_EPISODES")

		for _, char := range downloadNextNEpisodes {
			if !unicode.IsDigit(char) {
				fmt.Println("⚠️ Only digit are allowed in DOWNLOAD_NEXT_EPISODES")
			}
		}

		nextNEpisodes, err := strconv.ParseUint(downloadNextNEpisodes, 10, 16)
		if err != nil {
			fmt.Printf("⚠️ Error parsing %s: %s\n", downloadNextNEpisodes, err)
		}

		endEpisode := uint(selectedEpisode.Number) + uint(nextNEpisodes)
		fmt.Printf("End episode: %d\n", endEpisode)
		var episodes []models.Episode
		if toContinue {
			fmt.Printf("Continue watching episode %d\n", selectedEpisode.Number+1)
			selectedEpisode = models.Episode{
				Number: selectedEpisode.Number + 1,
			}

			// GET ONLY WHAT NEEDED N = SELECTED.NUMBER
			var err error
			episodes, err = animeUnityInstance.GetEpisodes(selectedSeries, uint(selectedEpisode.Number), endEpisode)

			if err != nil {
				fmt.Printf("⚠️ Error retriving episodes \n\t- %s\n", err)
				fmt.Println("Continue to watch locally")
			}
		} else {
			var err error
			episodes, err = animeUnityInstance.GetEpisodes(selectedSeries, 1, math.MaxUint)
			if err != nil {
				fmt.Printf("⚠️ Error retriving episodes \n\t- %s\n", err)
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

		pool := routinepool.GetInstance()

		pool.AddTask(func() {
			func(episode models.Episode) {
				fmt.Printf("⬇️ Downloading episode %d\n", episode.Number)
				path, error := animeUnityInstance.DownloadEpisode(episode, user.RootDir)

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


		// The iterator starts at 0, but the first episode has number = 1 and is at index 0 in the slice.
		// This means we can simply add the iterator to the number of episodes already downloaded —
		// the result will correctly match the index in the slice.
		fmt.Printf("⬇️ Downloading next %d episodes\n", nextNEpisodes)

		downloadNext := pool.AddSubGroup("download_next", uint(nextNEpisodes), 5)
		defer downloadNext.Close()

		for _, episode := range episodes {

			if episode.Number == selectedEpisode.Number || episode.Number < selectedEpisode.Number {
				continue
			}

			if uint(episode.Number) > endEpisode || nextNEpisodes == 0 {
				break
			}

			ep := episode
			downloadNext.AddTask(func() {
				fmt.Printf("⬇️ Downloading episode %d\n", ep.Number)

				_, error := animeUnityInstance.DownloadEpisode(ep, user.RootDir)

				if error != nil {
					fmt.Printf("⚠️ Error downloading episode %d: \n\t- %s\n", ep.Number, error)
					return
				}

				fmt.Printf("✅ Episode downloaded: %d\n", ep.Number)
			})

			nextNEpisodes--
		}

		if delete_prev {
			basePath := fmt.Sprintf("%s/%s", user.RootDir, selectedSeries.Slug)
			files, err := os.ReadDir(basePath)
			if err != nil {
				fmt.Printf("⚠️ Error reading directory to delete %s: \n\t- %s\n", basePath, err)
			} else {
				deletePrev := pool.AddSubGroup("delete_prev", uint(len(files)), 1)
				defer deletePrev.Close()
				for _, file := range files {
					f := file
					deletePrev.AddTask(func() {
						episodeNumber, err := strconv.Atoi(strings.Split(f.Name(), ".")[0])
						if err != nil {
							fmt.Printf("⚠️ Error parsing file name to delete %s: \n\t- %s\n", f.Name(), err)
							return
						}

						if episodeNumber >= int(selectedEpisode.Number) {
							return
						}

						fmt.Printf("❌ Deleting file %s\n", basePath+"/"+f.Name())

						if err := os.Remove(basePath + "/" + f.Name()); err != nil {
							fmt.Printf("⚠️ Error deleting file %s: \n\t- %s\n", basePath+"/"+f.Name(), err)
						}
					})
				}
			}
		}

		pool.WaitAll()

		// TODO: currently useless but filter must be updated on --serve version
	},
}

func init() {
	rootCmd.Flags( ).BoolVarP  ( &delete_prev   , "delete" , "d" , false , "Delete previus episodes")
	rootCmd.Flags( ).BoolVarP  ( &list          , "list"   , "l" , false , "Show list of following series")
	rootCmd.Flags( ).StringVarP( &series_title  , "title"  , "t" , ""    , "Series title")
	rootCmd.Flags( ).StringVarP( &userName      , "user"   , "u" , ""    , "User.env file for configuration loading")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func searchForSeries(animeUnityInstance *animeunity.AnimeUnity, title string) (models.Series, error) {

	if title == "" || len(title) == 0 {
		return models.Series{}, fmt.Errorf("please provide sires title with --title flag")
	}

	seriesList, err := animeUnityInstance.Search(title)
	if err != nil {
		return models.Series{}, err
	}

	if len(seriesList) == 0 {
		return models.Series{}, fmt.Errorf("no results found")
	}

	for i, v := range seriesList {
		fmt.Printf("%d - %s\n", i+1, v.Slug)
	}

	fmt.Println("Select a series")
	reader := bufio.NewReader(os.Stdin)

	selected, _ := reader.ReadString('\n')
	selected = strings.TrimSpace(selected)

	if selected == "" || len(selected) == 0 {
		return models.Series{}, fmt.Errorf("invalid selection")
	}

	for _, char := range selected {
		if !unicode.IsDigit(char) {
			return models.Series{}, fmt.Errorf("only digit are allowed")
		}
	}

	index_selected, err := strconv.ParseUint(selected, 10, 16)
	if err != nil {
		return models.Series{}, fmt.Errorf("invalid selection")
	}

	if index_selected < 1 || uint16(index_selected) > uint16(len(seriesList)) {
		return models.Series{}, fmt.Errorf("invalid selection")
	}

	return seriesList[index_selected-1], nil
}
