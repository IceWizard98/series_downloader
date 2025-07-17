package animeunity

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"

	"github.com/IceWizard98/series_downloader/models"
	"github.com/IceWizard98/series_downloader/models/httpclient"
	bloomfilter "github.com/IceWizard98/series_downloader/utils/bloomFilter"
	"github.com/IceWizard98/series_downloader/utils/routinepool"
	"github.com/PuerkitoBio/goquery"
)

type AnimeUnity struct {
	client   *httpclient.APIClient
	anime    anime
}

type anime struct {
	ID          uint   `json:"id"                  `
	Name        string `json:"title_eng"           `
	ImageURL    string `json:"imageurl"            `
	Episodes    uint   `json:"real_episodes_count" `
	Slug        string `json:"slug"                `
}

type episode struct {
	ID          uint   `json:"id"`
	Number      string `json:"number"`
	EpisodeCode uint   `json:"scws_id"`
}

/*
	Initializes a new AnimeUnity instance
*/
func Init() (*AnimeUnity, error) {
	fmt.Println("Initializing animeunity")

	instance := &AnimeUnity{}
	client, err := httpclient.NewAPIClient("https://www.animeunity.so", 5)

	if err != nil {
		return instance, fmt.Errorf("error creating animeunity http client: \n\t- %w", err)
	}
	
	instance.client = client
	
	if !instance.client.Initialized {
		err := instance.client.Initialize()
		if err != nil {
			instance.client = nil
			return instance, fmt.Errorf("error initializing animeunity http client: \n\t- %w", err)
		}
	}

	return instance, nil
}

/*
	Search for animes by title using the API endpoint
  The result is a list of models.Series
*/
func (a AnimeUnity) Search( query string ) ([]models.Series, error) {
	if a.client == nil {
		return make([]models.Series, 0), nil
	}

	search        := fmt.Sprintf(`{"title":"%s"}`, query)
	response, err := a.client.DoRequest("POST", "/livesearch", search)

	if err != nil {
		return nil, fmt.Errorf("error searching for %s: \n\t- %w", query, err)
	}
	
	if string(response) == "null" || response == nil {
		fmt.Println("Response is empty")
		return make([]models.Series, 0), nil
	}

	var res map[string]json.RawMessage
	err = json.Unmarshal(response, &res)
	if err != nil {
		return nil, fmt.Errorf("error searching for %s: \n\t- %w", query, err)
	}

	var animeList []anime
	err = json.Unmarshal(res["records"], &animeList)
	if err != nil {
		return nil, fmt.Errorf("error searching for %s: \n\t- %w", query, err)
	}

	var animeModels []models.Series
	for _, v := range animeList {
		animeModels = append(animeModels, models.Series{
			ID:       fmt.Sprintf("%d", v.ID),
			Name:     v.Name,
			ImageURL: v.ImageURL,
			Episodes: v.Episodes,
			Slug:     v.Slug,
		})
	}

	return animeModels, nil
}

/*
  Get the anime episodes using the API endpoint
	The result is a list of models.Episode
*/
func (a *AnimeUnity) GetEpisodes( animeModel models.Series, start uint, end uint ) ([]models.Episode, error) {
	numberId, _ := strconv.ParseUint(animeModel.ID, 10, 64)

	a.anime = anime{
		ID:       uint(numberId),
		Name:     animeModel.Name,
		ImageURL: animeModel.ImageURL,
		Episodes: animeModel.Episodes,
		Slug:     animeModel.Slug,
	}

	totEpisodes := a.anime.Episodes

	if totEpisodes == 0 {
		return make([]models.Episode, 0), nil
	}

	if a.client == nil {
		return make([]models.Episode, 0), nil
	}

	if !a.client.Initialized {
		err := a.client.Initialize()
		if err != nil {
			return nil, fmt.Errorf("error initializing client: \n\t- %w", err)
		}
	}

	pool := routinepool.GetInstance().AddSubGroup("animeunity", 100, 5)
	ch   := make(chan []byte)

	if end > totEpisodes || end == 0 {
		end = totEpisodes
	}

	for i := start; i <= end; i += 120 {
		pool.AddTask( func() {
			func(ch chan<- []byte, start uint) {
  	    response, err := a.client.DoRequest("GET", fmt.Sprintf("/info_api/%d/1?start_range=%d&end_range=%d", a.anime.ID, start, start+119), "")

  	    if err != nil {
		    	ch <- []byte("null")
  	    	return
  	    }
  
  	    ch <- response
			}(ch, i)
		})
	}

	go func() {
		pool.Wait()
    close(ch)
  }()

	var episodesList []models.Episode
	/*
		Iterate over the channel to get the episodes and convert them to models.Episode
	*/
	for res := range ch {
		if string(res) == "null" || res == nil || len(res) == 0 {
			return nil, fmt.Errorf("error searching for %s from %d to %d: \n\t- Response is empty", a.anime.Name, start, end)
		}

		var resultJson map[string]json.RawMessage
		err := json.Unmarshal(res, &resultJson)
	  if err != nil {
			return nil, fmt.Errorf("on base response unmarshal %s: \n\t- %w", a.anime.Name, err)
	  }

	  var episodesListChunk []episode
	  err = json.Unmarshal(resultJson["episodes"], &episodesListChunk)
	  if err != nil {
			return nil, fmt.Errorf("on unmarshal episodes %s: \n\t- %w", a.anime.Name, err)
	  }

	  for _, v := range episodesListChunk {
			episodeNumber, err := strconv.ParseUint(v.Number, 10, 16); if err != nil {
		    continue
	    }

	    number := uint16(episodeNumber)

			episode := models.Episode{
				ID:          v.ID,
				Number:      number,
				EpisodeCode: fmt.Sprintf("%d", v.EpisodeCode),
			}

			episodesList = append(episodesList, episode)
	  }
  }

	sort.Slice(episodesList, func(i, j int) bool {
		return episodesList[i].Number < episodesList[j].Number
	})

	return episodesList, nil
}

/*
	Download an episode using the API endpoint and save it to disk
*/
func (a AnimeUnity) DownloadEpisode(episode models.Episode, rootDir string) (string, error) {
	basePath := fmt.Sprintf("%s/%s", rootDir, a.anime.Slug)
	fileName := fmt.Sprintf("%d.mp4", episode.Number)
	fullPath := basePath + "/" + fileName

	filter := bloomfilter.GetInstance(100)

	if filter.Contains([]byte(fullPath)) {
		if _, err := os.Stat(fullPath); err == nil {
			return fullPath, nil
		}
	}

	if a.anime.ID == 0 {
		return "", errors.New("anime id is 0")
	}
	if episode.ID == 0 {
		return "", errors.New("episode id is 0")
	}
	if a.anime.Slug == "" {
		return "", errors.New("anime slug is empty")
	}

	response, err := a.client.DoRequest("GET", fmt.Sprintf("/anime/%d-%s/%d", a.anime.ID, a.anime.Slug, episode.ID), "")
	if err != nil {
		return "", err
	}
	if response == nil || string(response) == "null" {
		return "", errors.New("response is empty")
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(response))
	if err != nil {
		return "", err
	}

	var embedUrl string
	doc.Find("video-player").Each(func(i int, s *goquery.Selection) {
		url, exists := s.Attr("embed_url")
		if exists {
			embedUrl = url
		}
	})

	if embedUrl == "" {
		return "", errors.New("embed url not found")
	}

	req, err := http.NewRequest("GET", embedUrl, nil)
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("error doing request: %w", err)
	}
	defer resp.Body.Close()

	embedHtml, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %w", err)
	}
	if embedHtml == nil || string(embedHtml) == "null" {
		return "", errors.New("embed response is empty")
	}

	embedDoc, err := goquery.NewDocumentFromReader(bytes.NewReader(embedHtml))
	if err != nil {
		return "", fmt.Errorf("error creating document: %w", err)
	}

	// Estrazione URL di download
	var downloadUrl string
	embedDoc.Find("script").Each(func(i int, s *goquery.Selection) {
		content := s.Text()
		re := regexp.MustCompile(`window.downloadUrl\s*=\s*['"]([^"]+)['"]`)
		match := re.FindStringSubmatch(content)
		if len(match) > 1 {
			downloadUrl = match[1]
		}
	})

	if downloadUrl == "" {
		return "", errors.New("download url not found")
	}

	resp, err = http.Get(downloadUrl)
	if err != nil {
		return "", fmt.Errorf("error getting download url: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("invalid status code: %s", resp.Status)
	}

	err = os.MkdirAll(basePath, os.ModePerm)
	if err != nil {
		return "", fmt.Errorf("error creating directory: %w", err)
	}

	var shouldCleanup = false
	defer func() {
		if shouldCleanup {
			_ = os.Remove(fullPath)
		}
	}()

	outFile, err := os.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("error creating file: %w", err)
	}
	defer outFile.Close()
	shouldCleanup = true

	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		return "", fmt.Errorf("error copying file: %w", err)
	}

	shouldCleanup = false

	return fullPath, nil
}
