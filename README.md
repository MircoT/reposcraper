# reposcraper

A program to search into your own repositories from Github, Gitlab, and Bitbucket

## How to use it

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

### Requirements

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
