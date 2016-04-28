package main

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"golang.org/x/tools/blog/atom"

	"github.com/ChimeraCoder/anaconda"
	"github.com/joho/godotenv"
)

const (
	MaxStatuses = 200
	MaxEntries  = 50
)

func main() {
	log.SetFlags(log.Lshortfile)

	if len(os.Args) > 0 {
		if err := godotenv.Load(os.Args...); err != nil {
			log.Fatal(err)
		}
	}

	anaconda.SetConsumerKey(os.Getenv("TWITTER_CONSUMER_KEY"))
	anaconda.SetConsumerSecret(os.Getenv("TWITTER_CONSUMER_SECRET"))

	api := anaconda.NewTwitterApi(os.Getenv("TWITTER_OAUTH_TOKEN"), os.Getenv("TWITTER_OAUTH_TOKEN_SECRET"))
	defer api.Close()

	feedUrl := os.Getenv("FEED_URL")
	if feedUrl == "" {
		log.Fatal("FEED_URL is required.")
	}
	template := os.Getenv("TEMPLATE")
	if template == "" {
		log.Fatal("TEMPLATE is required.")
	}

	userId := strings.SplitN(os.Getenv("TWITTER_OAUTH_TOKEN"), "-", 2)[0]
	if _, err := strconv.ParseInt(userId, 10, 64); err != nil {
		log.Fatal(err)
	}

	var recentUrls map[string]struct{}
	{
		v := url.Values{}
		v.Set("user_id", userId)
		v.Set("count", string(MaxStatuses))
		timeline, err := api.GetUserTimeline(v)
		if err != nil {
			log.Fatal(err)
		}

	statuses:
		for _, status := range timeline {
			for _, url := range status.Entities.Urls {
				fmt.Println(url.Expanded_url)
				if _, ok := recentUrls[url.Expanded_url]; ok {
					continue statuses
				}
				recentUrls[url.Expanded_url] = struct{}{}
			}
		}
	}

	var atom atom.Feed
	{
		resp, err := http.Get(feedUrl)
		if err != nil {
			log.Fatal(err)
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatal(err)
		}

		if err := xml.Unmarshal(body, &atom); err != nil {
			log.Fatal(err)
		}
	}

	{
		for i, entry := range atom.Entry {
			if i >= MaxEntries {
				break
			}

			if len(entry.Link) == 0 {
				continue
			}
			link := entry.Link[0]
			if _, ok := recentUrls[link.Href]; ok {
				continue
			}

			replacer := strings.NewReplacer("{title}", entry.Title, "{url}", link.Href)
			text := replacer.Replace(template)
			_, err := api.PostTweet(text, url.Values{})
			if err != nil {
				log.Fatal(err)
			}
		}
	}
}
