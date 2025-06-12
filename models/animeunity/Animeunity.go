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
	"github.com/IceWizard98/series_downloader/utils/routinepoll"
	"github.com/PuerkitoBio/goquery"
)

type animeUnity struct {
	Client   *httpclient.APIClient
	anime    anime
}

type anime struct {
	ID          uint      `json:"id"`
	Name        string    `json:"title_eng"`
	ImageURL    string    `json:"imageurl"`
	Episodes    uint      `json:"real_episodes_count"`
	Slug        string    `json:"slug"`
}

type episode struct {
	ID          uint   `json:"id"`
	Number      string `json:"number"`
	EpisodeCode uint   `json:"scws_id"`
}

/*
	Initializes a new animeUnity instance
*/
func Init() *animeUnity {
	instance := &animeUnity{}
	client, err := httpclient.NewAPIClient("https://www.animeunity.so")

	if err != nil {
		panic(err)
	}
	
	instance.Client = client
	return instance
}

/*
	Search for animes by title using the API endpoint
  The result is a list of models.Serie
*/
func (a animeUnity) Search( query string ) ([]models.Serie, error) {
	search        := fmt.Sprintf(`{"title":"%s"}`, query)
	response, err := a.Client.DoRequest("POST", "/livesearch", search)

	if err != nil {
		return nil, fmt.Errorf("error searching for %s", query)
	}
	
	if string(response) == "null" || response == nil {
		fmt.Println("Response is empty")
		return make([]models.Serie, 0), nil
	}

	var res map[string]json.RawMessage
	err = json.Unmarshal(response, &res)
	if err != nil {
		return nil, fmt.Errorf("error searching for %s: %s", query, err)
	}

	var animeList []anime
	err = json.Unmarshal(res["records"], &animeList)
	if err != nil {
		return nil, fmt.Errorf("error searching for %s: %s", query, err)
	}

	var animeModels []models.Serie
	for _, v := range animeList {
		animeModels = append(animeModels, models.Serie{
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
func (a *animeUnity) GetEpisodes( animeModel models.Serie ) ([]models.Episode, error) {
	numberId, err := strconv.ParseUint(animeModel.ID, 10, 64); if err != nil {
		return nil, fmt.Errorf("error parsing id: %s", err)
	}

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

	pool := routinepoll.GetInstance()
	ch   := make(chan []byte)

	for i := uint(1); i <= totEpisodes; i += 120 {
		pool.AddTask( func() {
			func(ch chan<- []byte, start uint) {
  	    response, err := a.Client.DoRequest("GET", fmt.Sprintf("/info_api/%d/1?start_range=%d&end_range=%d", a.anime.ID, start, start+119), "")

  	    if err != nil {
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
	  var resultJson map[string]json.RawMessage
		err := json.Unmarshal(res, &resultJson)
	  if err != nil {
			return nil, fmt.Errorf("on base response unmarshal %s: %s", a.anime.Name, err)
	  }

	  var episodesListChunk []episode
	  err = json.Unmarshal(resultJson["episodes"], &episodesListChunk)
	  if err != nil {
			return nil, fmt.Errorf("on unmarshal episodes %s: %s", a.anime.Name, err)
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
func (a animeUnity) DownloadEpisode( episode models.Episode, rootDir string ) (string, error) {
  basePath := fmt.Sprintf(rootDir + "/%s", a.anime.Slug)
	fileName := fmt.Sprintf("%d.mp4", episode.Number)
	fullPath := basePath + "/" + fileName

	filter := bloomfilter.GetInstance()

	if filter.Contains([]byte(fullPath)) {
		if _, err := os.Stat(fullPath); err == nil {
			return fullPath, nil
		}
	}

  response, err := a.Client.DoRequest("GET", fmt.Sprintf("/anime/%d-%s/%d", a.anime.ID, a.anime.Slug, episode.ID), "")
  if err != nil {
  	return "", err
 	}

	if string(response) == "null" || response == nil {
		return "", errors.New("response is empty")
	}

	var error    error
	var embedUrl string

  doc, err := goquery.NewDocumentFromReader( bytes.NewReader(response) )
  if err != nil {
		return "", err
  }

	// find the embed url, that page conains the download url for the episode
  doc.Find("video-player").Each(func(i int, s *goquery.Selection) {
    url, exists := s.Attr("embed_url")
    if exists {
      embedUrl = url   
		}
  })

	if embedUrl == "" {
		error = errors.New("embed url not found")
		return "", error
	}

	var embedHtml []byte
	{
	  req, err := http.NewRequest("GET", embedUrl, nil)
	  if err != nil {
	  	error = err
	  	return "", error
	  }

	  resp, err := http.DefaultClient.Do(req)
	  if err != nil {
	  	error = err
	  	return "", error
	  }

	  defer resp.Body.Close()

		embedHtml, err = io.ReadAll(resp.Body)
	  if err != nil {
	  	error = err
	  	return "", error
	  }
	}

	if string(embedHtml) == "null" || embedHtml == nil {
		error = errors.New("embed response is empty")
	 	return "", error
	}

	embedDoc, err := goquery.NewDocumentFromReader( bytes.NewReader(embedHtml) )
	if err != nil {
		error = err
	  return "", error
	}

	// find the download url and use it to download the episond and saavi it into a file
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
		error = errors.New("download url not found")
	  return "", error
	}

	{
    resp, err := http.Get(downloadUrl)
    if err != nil {
			return "", err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("errore nella risposta: %s", resp.Status)
    }

		err = os.MkdirAll(basePath, os.ModePerm)
		if err != nil {
			return "", err
		}

    outFile, err := os.Create(fullPath)

    if err != nil {
			return "", err
    }
    defer outFile.Close()

    _, err = io.Copy(outFile, resp.Body)
    if err != nil {
			return "", err
    }
	}

	return fullPath, nil
}
