# reposcraper

A program to search for own repositories from Github, Gitlab, and Bitbucket written in Go.

## Get the program

```bash
go get github.com/MircoT/reposcraper
```

If you have correctly configured [Go](https://golang.org/), just use `reposcraper` from the command line and puts the configuration file in your home folder `$HOME/.reposcraper/config.json`. Read the following documentation for the [configuration file requirements](#config-file-requirements).

## From source

Edit the `config.example.json` and rename it `config.json`.
You have to put the information for each service you want to use.
For example, if you need only the GitHub search you have to add only this information:

```json
{
    "github": {
        "username": "your username",
        "token": "your token"
    },
}
```

After that, you can start to use the program simply by typing:

```bash
go run .
```

You can also build the executable and then use it:

```bash
go build
./reposcraper
```

After the program collected all the repositories information (name and description, if it is private or not),
you can type to search for a repository and select it from the suggestions. 
Select the correct response by pressing enter and the program will try to open the repository in the browser using its URL.

By default, the program search also the repository you starred.
To search for a specific type, use the flag `type`:

```
Search for a repository that you own or you starred

Usage:
  reposcraper [flags]

Flags:
      --config string   config file (default "./config.json")
  -h, --help            help for reposcraper
      --type string     search type: [all, starred, owned] (default "all")
```

### Config file requirements

The program needs the following token from the various services:

* GitHub: a [personal access token](https://github.com/settings/tokens) with the `repo` and `user` scopes.
* GitLab: a [personal access token](https://docs.gitlab.com/ee/user/profile/personal_access_tokens.html#personal-access-tokens) with the `api` and `read_repository` scopes
* Bitbucket: an [OAuth consumers](https://support.atlassian.com/bitbucket-cloud/docs/use-oauth-on-bitbucket-cloud/) with the following characteristics:
  * Callback URL: `http://localhost/bitbucket` (or whatever you want, it is not used but only required by the Bitbucket API)
  * `This is a private consumer` option checked
  * Permission on `Projects Read`

Then, you can compile the config file with the proper information:

```json
{
    "github": {
        "username": "user",
        "token": "yyy"
    },
    "gitlab": {
        "username": "user",
        "token": "zzz"
    },
    "bitbucket": {
        "username": "user",
        "key": "aaa",
        "secret": "bbb"
    }
}
```

> **Note**: if a service is missing it will not be used.

### Make your own binary

```bash
git clone https://github.com/MircoT/reposcraper.git
cd reposcraper
go build
```
