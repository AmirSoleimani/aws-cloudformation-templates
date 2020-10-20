package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"context"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/dghubble/go-twitter/twitter"
	"github.com/dghubble/oauth1"
)

var (
	consumerKey    = os.Getenv("consumerKey")
	consumerSecret = os.Getenv("consumerSecret")

	authToken  = os.Getenv("authToken")
	authSecret = os.Getenv("authSecret")
)

var (
	once         sync.Once
	client       *twitter.Client
	defaultTweet = []string{
		"Yea! Cat got my tongue.",
		"Nothing to say, TODAY!",
	}
)

type dailyQuote struct {
	Contents struct {
		Quotes []struct {
			Quote  string   `json:"quote"`
			Author string   `json:"author"`
			Tags   []string `json:"tags"`
		} `json:"quotes"`
	} `json:"contents"`
}

func init() {
	once.Do(func() {
		config := oauth1.NewConfig(consumerKey, consumerSecret)
		token := oauth1.NewToken(authToken, authSecret)

		httpClient := config.Client(oauth1.NoContext, token)
		client = twitter.NewClient(httpClient)

		// Verify Credentials
		verifyParams := &twitter.AccountVerifyParams{
			SkipStatus:   twitter.Bool(true),
			IncludeEmail: twitter.Bool(true),
		}

		user, _, err := client.Accounts.VerifyCredentials(verifyParams)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("User's ACCOUNT:\n%+v\n", user)
	})
}

func main() {
	lambda.Start(HandleRequest)
}

func getQuote() (*dailyQuote, error) {
	client := http.Client{
		Timeout: 2 * time.Second,
	}
	resp, err := client.Get("https://quotes.rest/qod?language=en")
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errors.New("response is empty")
	}
	defer resp.Body.Close()

	var result dailyQuote
	json.NewDecoder(resp.Body).Decode(&result)
	return &result, nil
}

// HandleRequest handle income request
func HandleRequest(ctx context.Context) error {
	quote, err := getQuote()
	if err != nil {
		return err
	}

	var tweet string
	if len(quote.Contents.Quotes) > 0 {
		tweet = fmt.Sprintf(`%s //%s`,
			quote.Contents.Quotes[0].Quote,
			quote.Contents.Quotes[0].Author,
		)

		if len(quote.Contents.Quotes[0].Tags) > 0 {
			if len(quote.Contents.Quotes[0].Tags) > 2 {
				quote.Contents.Quotes[0].Tags = quote.Contents.Quotes[0].Tags[:2]
			}

			for _, tag := range quote.Contents.Quotes[0].Tags {
				if len(tag)+len(tweet) > 280 {
					break
				}
				if strings.Contains(tweet, tag) {
					tweet = strings.Replace(tweet, tag, "#"+tag, 1)
					continue
				}

				if !strings.HasSuffix(tweet, `\n`) {
					tweet = tweet + "\n"
				}
				tweet = tweet + `#` + tag
			}
		}
	}

	fmt.Println(tweet)
	if len(tweet) == 0 || len(tweet) > 280 {
		log.Printf("long or short tweet: %s\n", tweet)
		tweet = defaultTweet[time.Now().Day()%len(defaultTweet)]
	}

	twt, resp, err := client.Statuses.Update(tweet, nil)
	if err != nil {
		return err
	}
	if resp != nil {
		log.Printf("response: %v\n", *resp)
	}
	if twt != nil {
		log.Printf("tweet: %v\n", *twt)
	}

	return nil
}
