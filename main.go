package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"github.com/samber/lo"
)

type ArticleResult struct {
	Title string `json:"title"`
	URL   string `json:"url"`
	score float64
}

// Returns body after http request to provided URL
func getBodyFromUrl(url string) ([]byte, error) {
	response, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP request failed with code: %d", response.StatusCode)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func fetchTitles(size int, articleIDs []int) ([]ArticleResult, error) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	// Handles case when there are not enought articles fetched
	if size > len(articleIDs) {
		size = len(articleIDs)
	}
	result := make([]ArticleResult, size)
	for i := 0; i < size; i++ {
		wg.Add(1)
		// TODO: Rewrite this to use channels and extract this function.
		go func(i int) {
			defer wg.Done()
			articleURL := "https://hacker-news.firebaseio.com/v0/item/"
			numString := strconv.Itoa(articleIDs[i])
			articleURL += numString
			articleURL += ".json"

			body, err := getBodyFromUrl(articleURL)
			if err != nil {
				fmt.Println("Error: ", err)
				return
			} else {
				var dataMap map[string]interface{}

				if err := json.Unmarshal(body, &dataMap); err != nil {
					fmt.Println("Error:", err)
					return
				}

				title, ok := dataMap["title"].(string)
				if !ok {
					fmt.Println("Error: 'title' key is not a string")
					return
				}
				url, ok := dataMap["url"].(string)
				if !ok {
					// fmt.Println("Error: 'url' key is not a string")
					return
				}

				score, ok := dataMap["score"].(float64)
				if !ok {
					fmt.Println("Error: 'score' key is not an int")
					return
				}
				mu.Lock()
				result[i] = ArticleResult{title, url, score}
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	return result, nil
}

func getHighestScore(titlesList []ArticleResult) int {
	highestScore := titlesList[0].score
	highestIndex := 0

	for num, score := range titlesList {
		if score.score > highestScore {
			highestScore = score.score
			highestIndex = num
		}
	}
	return highestIndex
}

var filter = flag.String("filter", "", "filter articles with specific keyword")
var number = flag.Int("number", 10, "number of articles to display")
var noInput = flag.Bool("no-input", false, "do not wait for user input, can be used for scripting")
var jsonOutput = flag.Bool("json", false, "output in json format")

func main() {
	flag.Parse()
	const DEFAULT_ARTICLE_NUMBER = 10
	if (!*noInput && !*jsonOutput) {
		fmt.Println("Hello Hackers News!")
	}
	body, err := getBodyFromUrl("https://hacker-news.firebaseio.com/v0/topstories.json")

	if err != nil {
		fmt.Printf("Error: %s\n", err)
		os.Exit(1)
	}
	var articleIDs []int
	if err := json.Unmarshal([]byte(body), &articleIDs); err != nil {
		fmt.Printf("Error: %s\n", err)
		os.Exit(1)
	}

	titlesList, err := fetchTitles(*number, articleIDs)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		os.Exit(1)
	}

	if *filter != "" {
		titlesList = lo.Filter(titlesList, func(art ArticleResult, index int) bool {
			return strings.Contains(art.Title, *filter)
		})
	}

	if *noInput {
		for _, value := range titlesList {
			// For some reason fmt.Println(value.title) does not work with wc -l
			fmt.Printf("%s ~ %s\n", value.Title, value.URL)
		}
		return
	}

	if *jsonOutput {
		jsonString, err := json.Marshal(titlesList)
		if err != nil {
			panic(err)
		}
		fmt.Println(string(jsonString))
		return
	}

	if len(titlesList) == 0 {
		fmt.Println("No articles found 🧐")
		return
	}

	highestIndex := getHighestScore(titlesList)
	for num, value := range titlesList {
		if num == highestIndex {
			fmt.Printf("%d: %s ⭐\n", num+1, value.Title)
		} else {
			fmt.Printf("%d: %s\n", num+1, value.Title)
		}
	}

	fmt.Print("🔗:")

	var number int64
	_, err = fmt.Scanf("%d", &number)

	if err != nil {
		fmt.Println("Could not get URL if the article 😞")
		os.Exit(1)
	}
	if int(number) > len(titlesList) || number < 1 {
		fmt.Println("There is no such article 🤦")
		os.Exit(1)
	}
	fmt.Println(titlesList[number-1].URL)
	exec.Command("firefox", titlesList[number-1].URL).Run()
}
