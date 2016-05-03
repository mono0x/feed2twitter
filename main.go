package main

import (
	"encoding/xml"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/tools/blog/atom"

	"github.com/ChimeraCoder/anaconda"
	"github.com/joho/godotenv"
)

const (
	MaxStatuses = 200
	MaxEntries  = 100
)

type atomEntrySlice []*atom.Entry

func (a atomEntrySlice) Len() int {
	return len(a)
}

func (a atomEntrySlice) Less(i, j int) bool {
	it, err := time.Parse(time.RFC3339, string(a[i].Updated))
	if err != nil {
		return false
	}
	jt, err := time.Parse(time.RFC3339, string(a[j].Updated))
	if err != nil {
		return false
	}
	return it.After(jt)
}

func (a atomEntrySlice) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

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

	recentUrls := make(map[string]struct{})
	{
		v := url.Values{}
		v.Set("user_id", userId)
		v.Set("count", strconv.Itoa(MaxStatuses))
		timeline, err := api.GetUserTimeline(v)
		if err != nil {
			log.Fatal(err)
		}

		for _, status := range timeline {
			for _, url := range status.Entities.Urls {
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
		entries := atomEntrySlice(atom.Entry)
		sort.Sort(sort.Reverse(entries))
		n := 0
		if len(entries) > MaxEntries {
			n = len(entries) - MaxEntries
		}
	entries:
		for _, entry := range entries[n:] {
			if len(entry.Link) == 0 {
				continue
			}
			link := entry.Link[0]
			if _, ok := recentUrls[link.Href]; ok {
				continue
			}

			replacer := strings.NewReplacer("{title}", entry.Title, "{url}", link.Href)
			text := replacer.Replace(template)
			if _, err := api.PostTweet(text, url.Values{}); err != nil {
				if apiErr, ok := err.(*anaconda.ApiError); ok {
					for _, err := range apiErr.Decoded.Errors {
						if err.Code == anaconda.TwitterErrorStatusIsADuplicate {
							continue entries
						}
					}
					log.Fatal(apiErr)
				} else {
					log.Fatal(err)
				}
			}
		}
	}
}
