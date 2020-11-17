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
	"runtime"
	"strings"
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
	cfgFile    string            //nolint:gochecknoglobals
	searchType string            //nolint:gochecknoglobals
	rootCmd    = &cobra.Command{ //nolint:exhaustivestruct,gochecknoglobals
		Use:   "reposcraper",
		Short: "A program to search into your own repositories from Github, Gitlab, and Bitbucket.",
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
				fmt.Println("Opening " + selection + " ...")
				scraper.OpenURL(selection)
			} else {
				fmt.Println("What you're searching for is not there...")
			}

			return nil
		},
	}
)

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			panic(err)
		}

		viper.AddConfigPath(home)
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}

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

func init() { //nolint: gochecknoinits
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "./config.json", "config file")
	rootCmd.PersistentFlags().StringVar(&searchType, "type", "all", "search type: [all, starred, owned]")

	viper.SetConfigName("config")
	viper.SetConfigType("json")
	viper.AddConfigPath("$HOME/.reposcraper")
	viper.AddConfigPath(".")
}

type ScraperRepo interface {
	GetName() string
	GetDescription() string
	GetURL() string
	GetVisibility() string
	GetType() string
}

type ServiceConfig struct {
	Username string `json:"username"`
	Token    string `json:"token"`
	Key      string `json:"key"`
	Secret   string `json:"secret"`
}

type Config struct {
	GitHub    *ServiceConfig `json:"github,omitempty"`
	GitLab    *ServiceConfig `json:"gitlab,omitempty"`
	Bitbucket *ServiceConfig `json:"bitbucket,omitempty"`
}

type Scraper struct {
	config       Config
	Repositories []ScraperRepo
	Counters     struct {
		GitHub    int
		GitLab    int
		Bitbucket int
	}
}

func openbrowser(url string) {
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

func (s Scraper) OpenURL(name string) {
	for _, repo := range s.Repositories {
		if repo.GetName() == name {
			openbrowser(repo.GetURL())
		}
	}
}

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

func (s *Scraper) insert(repo ScraperRepo) {
	s.Repositories = append(s.Repositories, repo)
}

func (s *Scraper) Collect() { //nolint:funlen
	github := make(chan ScraperRepo)
	githubStarred := make(chan ScraperRepo)
	gitlab := make(chan ScraperRepo)
	gitlabStarred := make(chan ScraperRepo)
	bitbucket := make(chan ScraperRepo)
	githubDone := false
	githubStarredDone := false
	gitlabDone := false
	gitlabStarredDone := false
	bitbucketDone := false

	switch searchType {
	case "all", "ALL":
		go s.gitHubRepos(github, false)
		go s.gitLabRepos(gitlab, false)
		go s.bitbucketRepos(bitbucket)
		go s.gitHubRepos(githubStarred, true)
		go s.gitLabRepos(gitlabStarred, true)

	case "owned", "OWNED":
		go s.gitHubRepos(github, false)
		go s.gitLabRepos(gitlab, false)
		go s.bitbucketRepos(bitbucket)

		githubStarredDone = true
		gitlabStarredDone = true
	case "starred", "STARRED":
		go s.gitHubRepos(githubStarred, true)
		go s.gitLabRepos(gitlabStarred, true)

		githubDone = true
		gitlabDone = true
		bitbucketDone = true
	}

	for !githubDone || !gitlabDone || !bitbucketDone || !githubStarredDone || !gitlabStarredDone {
		select {
		case repo, ok := <-github:
			if ok {
				s.insert(repo)
				s.Counters.GitHub++
			} else {
				githubDone = true
			}
		case repo, ok := <-githubStarred:
			if ok {
				s.insert(repo)
				s.Counters.GitHub++
			} else {
				githubStarredDone = true
			}
		case repo, ok := <-gitlab:
			if ok {
				s.insert(repo)
				s.Counters.GitLab++
			} else {
				gitlabDone = true
			}
		case repo, ok := <-gitlabStarred:
			if ok {
				s.insert(repo)
				s.Counters.GitLab++
			} else {
				gitlabStarredDone = true
			}
		case repo, ok := <-bitbucket:
			if ok {
				s.insert(repo)
				s.Counters.Bitbucket++
			} else {
				bitbucketDone = true
			}
		}
	}
}

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

func (s *Scraper) bitbucketRepos(c chan ScraperRepo) {
	/* Source:
	   - https://developer.atlassian.com/bitbucket/api/2/reference/meta/authentication
	   - https://support.atlassian.com/bitbucket-cloud/docs/use-oauth-on-bitbucket-cloud/
	   - https://support.atlassian.com/bitbucket-cloud/docs/oauth-consumer-examples/
	*/
	defer close(c)

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

func (s *Scraper) gitLabRepos(c chan ScraperRepo, starred bool) {
	defer close(c)

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

func (s *Scraper) gitHubRepos(c chan ScraperRepo, starred bool) { //nolint:funlen
	/* Source
	   - https://docs.github.com/en/free-pro-team@latest/rest/reference/repos#list-repositories-for-the-authenticated-user
	   - https://docs.github.com/en/free-pro-team@latest/github/authenticating-to-github/creating-a-personal-access-token
	*/
	defer close(c)

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
