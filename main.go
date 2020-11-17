package main

import (
	"github.com/MircoT/reposcraper/cmd"
)

func main() {
	cmd.Execute()
}

// func main() {
// 	scraper := Scraper{}
// 	spin := spinner.New(spinner.CharSets[28], spinnerDt)

// 	spin.Suffix = " Load config"
// 	spin.Start()

// 	errLoadConfig := scraper.LoadConfig("config.json")

// 	if errLoadConfig != nil {
// 		panic(errLoadConfig)
// 	}

// 	spin.Suffix = " Collecting repositories"

// 	scraper.Collect()
// 	spin.Stop()

// 	fmt.Printf("Found %d repositories...\n", len(scraper.Repositories))

// 	selection := prompt.Input("Search: ", wrapCompleter(scraper))

// 	if selection != "" {
// 		fmt.Println("Opening " + selection)
// 		scraper.OpenURL(selection)
// 	} else {
// 		fmt.Println("What you're searching for is not there...")
// 	}

// }
