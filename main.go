package main

import (
	"context"
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
	"time"

	"github.com/samber/lo"
)

type ArticleResult struct {
	Title string  `json:"title"`
	URL   string  `json:"url"`
	Score float64 // unexported field, should be Score
}

func getBodyFromUrl(url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*maxTime)*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	response, err := http.DefaultClient.Do(req)
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

func fetchTitles(ctx context.Context, size int, articleIDs []int) ([]ArticleResult, error) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	// Handles case when there are not enough articles fetched
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

			select {
			case <-ctx.Done():
				return // exit if the context is canceled
			default:
				body, err := getBodyFromUrl(articleURL)
				if err != nil {
					// fmt.Println("Error: ", err)
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

					Score, ok := dataMap["score"].(float64)
					if !ok {
						fmt.Println("Error: 'Score' key is not an int")
						return
					}
					mu.Lock()
					result[i] = ArticleResult{title, url, Score}
					mu.Unlock()
				}
			}
		}(i)
	}
	wg.Wait()
	return result, nil
}

func getHighestScore(titlesList []ArticleResult) int {
	highestScore := titlesList[0].Score
	highestIndex := 0

	for num, Score := range titlesList {
		if Score.Score > highestScore {
			highestScore = Score.Score
			highestIndex = num
		}
	}
	return highestIndex
}

var (
	filter     = flag.String("filter", "", "filter articles with specific keyword")
	number     = flag.Int("number", 10, "number of articles to display")
	noInput    = flag.Bool("no-input", false, "do not wait for user input, can be used for scripting")
	jsonOutput = flag.Bool("json", false, "output in json format")
	maxTime    = flag.Int("max-time", 10, "maximum time to wait for article")
)

func main() {
	flag.Parse()
	const DEFAULT_ARTICLE_NUMBER = 10
	if !*noInput && !*jsonOutput {
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

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*maxTime)*time.Second)
	defer cancel()

	titlesList, err := fetchTitles(ctx, *number, articleIDs)
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
