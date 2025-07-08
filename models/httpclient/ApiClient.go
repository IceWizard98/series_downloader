package httpclient

import (
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

type APIClient struct {
	BaseURL     string
	Client      *http.Client
	CSRFToken   string
	Initialized bool
}

func NewAPIClient(baseURL string, timeout uint8) (*APIClient, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("error creating cookie jar: \n\t- %s", err)
	}
	
	return &APIClient{
		BaseURL:   baseURL,
		Client:    &http.Client{Jar: jar, Timeout: time.Duration(timeout) * time.Second},
		Initialized: false,
	}, nil
}

func (a *APIClient) Initialize() error {
	resp, err := a.Client.Get(a.BaseURL)
	if err != nil {
		return fmt.Errorf("error initializing client: \n\t- %s", err)
	}
	defer resp.Body.Close()
	
	u, _ := url.Parse(a.BaseURL)
	cookies := a.Client.Jar.Cookies(u)
	for _, c := range cookies {
		if c.Name == "XSRF-TOKEN" {
			a.CSRFToken, _ = url.QueryUnescape(c.Value)
			break
		}
	}
	
	if a.CSRFToken == "" {
		return fmt.Errorf("error initializing client: \n\t- CSRF token not found")
	}
	
	a.Initialized = true
	return nil
}

func (a *APIClient) DoRequest(method, endpoint string, data string) ([]byte, error) {
	if !a.Initialized {
		if err := a.Initialize(); err != nil {
			return nil, fmt.Errorf("do request: \n\t- %s", err)
		}
	}
	
	var req *http.Request
	var err error
	
	if data != "" {
		req, err = http.NewRequest(method, a.BaseURL+endpoint, strings.NewReader(data))
	} else {
		req, err = http.NewRequest(method, a.BaseURL+endpoint, nil)
	}
	
	if err != nil {
		return nil, fmt.Errorf("do request: \n\terror creating request: \n\t- %s", err)
	}
	
	if data != "" {
		req.Header.Set("Content-Type", "application/json;charset=UTF-8")
	}
	req.Header.Set("X-XSRF-TOKEN", a.CSRFToken)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Origin", a.BaseURL)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	
	resp, err := a.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error doing request: \n\t- %s", err)
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: \n\t- %s", err)
	}
	
	return body, nil
}
