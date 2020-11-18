package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/briandowns/spinner"
	"github.com/c-bata/go-prompt"
	"github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	spinnerDt = 200 * time.Millisecond
)

var (
	// Used for flags.
	cfgFile    string //nolint:gochecknoglobals
	searchType string //nolint:gochecknoglobals

	// rootCmd the reposcraper command
	rootCmd = &cobra.Command{ //nolint:exhaustivestruct,gochecknoglobals
		Use:   "reposcraper",
		Short: "A program to search for own repositories from Github, Gitlab, and Bitbucket written in Go.",
		RunE: func(cmd *cobra.Command, args []string) error {
			scraper := Scraper{}
			spin := spinner.New(spinner.CharSets[28], spinnerDt)

			spin.Suffix = " Load config"
			spin.Start()

			errUnmar := viper.Unmarshal(&scraper.config)
			if errUnmar != nil {
				panic(errUnmar)
			}

			spin.Suffix = " Collecting repositories"
			scraper.Collect()
			spin.Stop()

			fmt.Printf("Found %d repositories...\n", len(scraper.Repositories))
			fmt.Printf("=> %d on GitHub\n=> %d on GitLab\n=> %d on Bitbucket\n",
				scraper.Counters.GitHub,
				scraper.Counters.GitLab,
				scraper.Counters.Bitbucket,
			)

			selection := prompt.Input("What are you searching for? > ", wrapCompleter(scraper))

			if selection != "" {
				fmt.Printf("Opening %s -> %s\n", selection, scraper.selectedURL(selection))
				openBrowser(scraper.selectedURL(selection))
			} else {
				fmt.Println("What you're searching for is not there...")
			}

			return nil
		},
	}
)

// init of the cobra root command and viper configuration
func init() { //nolint: gochecknoinits
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "./config.json", "config file")
	rootCmd.PersistentFlags().StringVar(&searchType, "type", "all", "search type: [all, starred, owned]")

	viper.SetConfigName("config")
	viper.SetConfigType("json")

	home, err := homedir.Dir()
	if err != nil {
		panic(err)
	}

	viper.AddConfigPath(path.Join(home, ".reposcraper"))
	viper.AddConfigPath(".")
}

// initConfig of viper
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
			if cfgFile != "./config.json" {
				fmt.Printf("WARNING: Configuration file '%s' not exists\n", cfgFile)
			}
		} else {
			viper.SetConfigFile(cfgFile)
		}
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	} else {
		fmt.Println("WARNING: No configuration file found...")
		fmt.Println(err)
		os.Exit(-1)
	}
}

// Execute of the reposcraper command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// wrapCompleter creates the suggestions from the repositories of the Scraper
func wrapCompleter(scraper Scraper) func(d prompt.Document) []prompt.Suggest {
	newFunc := func(d prompt.Document) []prompt.Suggest {
		suggestions := make([]prompt.Suggest, 0, len(scraper.Repositories))

		for _, repo := range scraper.Repositories {
			description := fmt.Sprintf("[%s](%s) %s",
				repo.GetType(),
				repo.GetVisibility(),
				repo.GetDescription(),
			)

			suggestions = append(suggestions, prompt.Suggest{
				Text: repo.GetName(), Description: description,
			})
		}

		return prompt.FilterFuzzy(suggestions, d.GetWordBeforeCursor(), true)
	}

	return newFunc
}

// ScraperRepo interface contract for a service
type ScraperRepo interface {
	GetName() string
	GetDescription() string
	GetURL() string
	GetVisibility() string
	GetType() string
}

// ServiceConfig for a single service of the Scraper
type ServiceConfig struct {
	Username string `json:"username"`
	Token    string `json:"token"`
	Key      string `json:"key"`
	Secret   string `json:"secret"`
}

// Config of the Scraper
type Config struct {
	GitHub    *ServiceConfig `json:"github,omitempty"`
	GitLab    *ServiceConfig `json:"gitlab,omitempty"`
	Bitbucket *ServiceConfig `json:"bitbucket,omitempty"`
}

// Scraper base structure
type Scraper struct {
	config       Config
	Repositories []ScraperRepo
	Counters     struct {
		GitHub    int
		GitLab    int
		Bitbucket int
	}
}

// openBrowser opens the system browser with the given URL
func openBrowser(url string) {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		panic("ERROR: Cannot open URL with the browuser -> unsupported platform!")
	}

	if err != nil {
		panic(err)
	}
}

// selectedURL returns the URL to open from the user selection
func (s Scraper) selectedURL(name string) string {
	resultURL := ""

	for _, repo := range s.Repositories {
		if repo.GetName() == name {
			resultURL = repo.GetURL()

			break
		}
	}

	return resultURL
}

// LoadConfig loads manually the Scraper configuration file (without using viper)
func (s *Scraper) LoadConfig(filename string) error {
	bytes, errIO := ioutil.ReadFile(filename)

	if errIO != nil {
		return errors.Wrap(errIO, "Scraper cannot load config")
	}

	s.config = Config{}

	errUnmarshal := json.Unmarshal(bytes, &s.config)

	if errUnmarshal != nil {
		return errors.Wrap(errUnmarshal, "Scraper cannot unmarshal config")
	}

	return nil
}

// insert a repo into the Scraper list
func (s *Scraper) insert(repo ScraperRepo) {
	s.Repositories = append(s.Repositories, repo)
}

// Collect all the repositories owned and starred by the user
func (s *Scraper) Collect() { //nolint:funlen
	buffer := make(chan ScraperRepo, 256)
	wg := sync.WaitGroup{}

	switch searchType {
	case "all", "ALL":
		wg.Add(5)

		go s.gitHubRepos(buffer, &wg, false)
		go s.gitLabRepos(buffer, &wg, false)
		go s.bitbucketRepos(buffer, &wg)
		go s.gitHubRepos(buffer, &wg, true)
		go s.gitLabRepos(buffer, &wg, true)
	case "owned", "OWNED":
		wg.Add(3)

		go s.gitHubRepos(buffer, &wg, false)
		go s.gitLabRepos(buffer, &wg, false)
		go s.bitbucketRepos(buffer, &wg)
	case "starred", "STARRED":
		wg.Add(2)

		go s.gitHubRepos(buffer, &wg, true)
		go s.gitLabRepos(buffer, &wg, true)
	}

	go func(c chan ScraperRepo, wg *sync.WaitGroup) {
		defer close(c)
		wg.Wait()
	}(buffer, &wg)

	for repo := range buffer {
		s.insert(repo)

		switch repo.GetType() {
		case "GitHub":
			s.Counters.GitHub++
		case "GitLab":
			s.Counters.GitLab++
		case "Bitbucket":
			s.Counters.Bitbucket++
		}
	}
}

// bitbucketAllRepos returns all the repositories from bitbucket
func bitbucketAllRepos(oauth BitbucketToken, config ServiceConfig) []BitbucketRepo {
	tURL, errURLT := template.New("bitbucket-url").Parse("https://bitbucket.org/api/2.0/repositories/{{.Username}}")

	if errURLT != nil {
		panic(errURLT)
	}

	ScraperURL := bytes.Buffer{}

	tExe := tURL.Execute(&ScraperURL, config)
	if tExe != nil {
		panic(tExe)
	}

	curResp := BitbucketResp{}
	curResp.Next = ScraperURL.String()
	prevURL := ""

	result := make([]BitbucketRepo, 0)

	for prevURL != curResp.Next {
		prevURL = curResp.Next

		req, errGet := http.NewRequestWithContext(context.Background(), "GET", curResp.Next, nil)
		if errGet != nil {
			panic(errGet)
		}

		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", oauth.AccessToken))

		resp, errResp := http.DefaultClient.Do(req)
		if errResp != nil {
			panic(errResp)
		}
		defer resp.Body.Close()

		body, errReadBody := ioutil.ReadAll(resp.Body)
		if errReadBody != nil {
			panic(errReadBody)
		}

		if resp.StatusCode != http.StatusOK {
			panic(fmt.Sprintf("Unexpected status code %d", resp.StatusCode))
		}

		errUnmarshal := json.Unmarshal(body, &curResp)

		if errUnmarshal != nil {
			panic(errUnmarshal)
		}

		result = append(result, curResp.Values...)
	}

	return result
}

// bitbucketRepos makes the token, starts the repository collections and sends them to the collector
func (s *Scraper) bitbucketRepos(c chan ScraperRepo, wg *sync.WaitGroup) {
	/* Source:
	   - https://developer.atlassian.com/bitbucket/api/2/reference/meta/authentication
	   - https://support.atlassian.com/bitbucket-cloud/docs/use-oauth-on-bitbucket-cloud/
	   - https://support.atlassian.com/bitbucket-cloud/docs/oauth-consumer-examples/
	*/
	defer wg.Done()

	repositories := make([]BitbucketRepo, 0)

	if s.config.Bitbucket != nil { // nolint:nestif
		reqBody := strings.NewReader(`grant_type=client_credentials`)

		req, errPost := http.NewRequestWithContext(context.Background(), "POST", bitBucketAccessTokenURL, reqBody)
		if errPost != nil {
			panic(errPost)
		}

		req.SetBasicAuth(s.config.Bitbucket.Key, s.config.Bitbucket.Secret)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, errResp := http.DefaultClient.Do(req)
		if errResp != nil {
			panic(errResp)
		}
		defer resp.Body.Close()

		body, errReadBody := ioutil.ReadAll(resp.Body)
		if errReadBody != nil {
			panic(errReadBody)
		}

		if resp.StatusCode != http.StatusOK {
			panic(fmt.Sprintf("Unexpected status code %d", resp.StatusCode))
		}

		oauthInfo := BitbucketToken{}
		errUnmarshal := json.Unmarshal(body, &oauthInfo)

		if errUnmarshal != nil {
			panic(errUnmarshal)
		}

		repositories = bitbucketAllRepos(oauthInfo, *s.config.Bitbucket)
	}

	for _, repo := range repositories {
		c <- repo
	}
}

// gitLabRepos collects all the GitLab repositories, starred and owned
func (s *Scraper) gitLabRepos(c chan ScraperRepo, wg *sync.WaitGroup, starred bool) { //nolint:funlen
	defer wg.Done()

	repositories := make([]GitLabRepo, 0)

	if s.config.GitLab != nil { // nolint:nestif
		var (
			tURL    *template.Template
			errURLT error
		)

		if !starred {
			tURL, errURLT = template.New("gitlab-url").Parse("https://gitlab.com/api/v4/users/{{.Username}}/projects")
		} else {
			// Source: https://docs.gitlab.com/ee/api/projects.html#list-projects-starred-by-a-user
			tURL, errURLT = template.New("gitlab-url").Parse("https://gitlab.com/api/v4/users/{{.Username}}/starred_projects")
		}

		if errURLT != nil {
			panic(errURLT)
		}

		ScraperURL := bytes.Buffer{}

		tExe := tURL.Execute(&ScraperURL, s.config.GitLab)
		if tExe != nil {
			panic(tExe)
		}

		req, errR := http.NewRequestWithContext(context.Background(), "GET", ScraperURL.String(), nil)

		if errR != nil {
			panic(errR)
		}

		req.Header.Add("PRIVATE-TOKEN", s.config.GitLab.Token)

		resp, errResp := http.DefaultClient.Do(req)
		if errResp != nil {
			panic(errResp)
		}

		defer resp.Body.Close()

		body, errReadBody := ioutil.ReadAll(resp.Body)
		if errReadBody != nil {
			panic(errReadBody)
		}

		if resp.StatusCode != http.StatusOK {
			panic(fmt.Sprintf("Unexpected status code %d", resp.StatusCode))
		}

		errUnmarshal := json.Unmarshal(body, &repositories)

		if errUnmarshal != nil {
			panic(errUnmarshal)
		}
	}

	for _, repo := range repositories {
		c <- repo
	}
}

// gitHubRepos collects all the GitHub repositories, starred and owned
func (s *Scraper) gitHubRepos(c chan ScraperRepo, wg *sync.WaitGroup, starred bool) { //nolint:funlen,gocognit
	/* Source
	   - https://docs.github.com/en/free-pro-team@latest/rest/reference/repos#list-repositories-for-the-authenticated-user
	   - https://docs.github.com/en/free-pro-team@latest/github/authenticating-to-github/creating-a-personal-access-token
	*/
	defer wg.Done()

	repositories := make([]GitHubRepo, 0)

	if s.config.GitHub != nil { // nolint:nestif
		var ScraperURL string

		page := 0

		for {
			page++

			tmp := make([]GitHubRepo, 0)

			tToken, errTokT := template.New("token").Parse("token {{.Token}}")
			if errTokT != nil {
				panic(errTokT)
			}

			if !starred {
				ScraperURL = fmt.Sprintf("https://api.github.com/user/repos?per_page=100&page=%d", page)
			} else {
				ScraperURL = fmt.Sprintf("https://api.github.com/user/starred?per_page=100&page=%d", page)
			}

			TokenHeader := bytes.Buffer{}

			tExe := tToken.Execute(&TokenHeader, s.config.GitHub)
			if tExe != nil {
				panic(tExe)
			}

			req, errR := http.NewRequestWithContext(context.Background(), "GET", ScraperURL, nil)

			if errR != nil {
				panic(errR)
			}

			req.SetBasicAuth(s.config.GitHub.Username, s.config.GitHub.Token)
			req.Header.Add("Accept", "application/vnd.github.v3+json")

			resp, errResp := http.DefaultClient.Do(req)
			if errResp != nil {
				panic(errResp)
			}

			defer resp.Body.Close()

			body, errReadBody := ioutil.ReadAll(resp.Body)
			if errReadBody != nil {
				panic(errReadBody)
			}

			if resp.StatusCode != http.StatusOK {
				panic(fmt.Sprintf("Unexpected status code %d", resp.StatusCode))
			}

			errUnmarshal := json.Unmarshal(body, &tmp)

			if errUnmarshal != nil {
				panic(errUnmarshal)
			}

			if len(tmp) > 0 {
				repositories = append(repositories, tmp...)
			} else {
				break
			}
		}
	}

	for _, repo := range repositories {
		c <- repo
	}
}
