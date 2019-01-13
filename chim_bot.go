package main

import (
	"encoding/json"
	"flag"
	"github.com/ChimeraCoder/anaconda"
	"github.com/davidk/lru"
	"github.com/davidk/memberset"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var (
	// Git release. Set on build with:
	// go install -ldflags "-X main.gitCommit=$(git describe
	// --abbrev=4 --dirty --always --tags)"
	gitCommit string

	config AppConfiguration

	api *anaconda.TwitterApi

	// LRU to avoid hitting rate limited Twitter API calls when we check
	// if a user is following a configured target
	tweetOriginatorLRU *lru.Cache

	// LRU to rate limit content
	userContentDeltaLRU *lru.Cache = lru.New(27)

	// LRU to allow time deltas between approved posts
	// we may refuse a post if results are within a certain delta rate
	userPostDeltaLRU *lru.Cache

	// Store the post text in a LRU cache to avoid spamming
	// the same message in a repeat fashion
	postTextLRU *lru.Cache

	// Track URL assets that we retweet, so duplicate tweets
	// that change the message slightly with the same content are not
	// retweeted
	urlLRU *lru.Cache

	// IDs that are muted. We check against this list and deny anyone on it.
	mutedIds *memberset.MemberSet = memberset.New()

	// Content types that are gated against a separate delta from the primary
	// post delta
	deltaGatedContent *memberset.MemberSet = memberset.New()

	// Single words and/or mentions that are blocked from being retweeted
	prohibitedMentions *memberset.MemberSet = memberset.New()
	prohibitedWords    *memberset.MemberSet = memberset.New()

	// Prometheus variables (metrics)
	tweetsProcessed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tweets_processed",
			Help: "Number of tweets processed.",
		},
		// type == filter method that allowed or denied tweet
		// result == result of hitting type
		[]string{"type", "result"},
	)
)

// AppConfiguration holds private credential data from config.json
// and varying data for different applications/communities
type AppConfiguration struct {
	ConsumerKey       string         `json:"consumer_key"`
	ConsumerSecret    string         `json:"consumer_secret"`
	AccessToken       string         `json:"access_token"`
	AccessTokenSecret string         `json:"access_token_secret"`
	SearchTerms       string         `json:"search_terms"`
	WatchUsers        string         `json:"watch_users"`
	LogrusLevel       string         `json:"logrus_level"`
	Settings          InternalTuning `json:"settings"`
	TestMode          bool           `json:"test_mode"`
}

// InternalTuning consists of behaviour tunables for very basic spam/anti-abuse
type InternalTuning struct {
	MustFollow           string   `json:"must_follow"`
	IgnoreFrom           string   `json:"ignore_from"`
	PostTimeDelta        int      `json:"post_time_delta_seconds"`
	ContentTimeDelta     int      `json:"delta_gated_content_time_seconds"`
	DeltaGatedContent    []string `json:"delta_gated_content"`
	DenySensitiveContent bool     `json:"deny_sensitive_content"`
	MinAccountAgeHours   int      `json:"min_account_age_hours"`
	MutualFollow         bool     `json:"mutual_follow"`
	ProhibitedMentions   []string `json:"prohibited_mentions"`
	ProhibitedWords      []string `json:"prohibited_words"`

	// TwitterFilterLevel is a twitter internal bit used by their ML
	// to make content displayable in public. Currently most tweets
	// we see are at the very least 'low'
	// filter_level: none, low, medium, high (high not implemented)
	TwitterFilterLevel string `json:"twitter_filter_level"`
}

// Interfaces used to switch between production and testing environments
// chim_test.go has its own interfaces to Fatal which do not trigger
// an application crash
type ErrorInterface interface {
	Fatal(format string, v ...interface{})
}

type ErrorsAreFatal struct{}

func (fs ErrorsAreFatal) Fatal(format string, v ...interface{}) {
	log.Fatal(format, v)
}

// checkRetweetErrors looks to see if we can recover from an error
// that occurs during a retweet. If we can continue, try to do so,
// otherwise, crash out. If we do return safely, set the bool flag to true
// so that we can use this in testing.
func checkRetweetErrors(fs ErrorInterface, explain string, err error) bool {
	if err != nil {

		if e, ok := err.(*anaconda.ApiError); !ok {
			// Unhandled: had a result where this was a *json.SyntaxError instead.
			// Not sure if it was/is recoverable.
			log.Println("Unable to convert error to *anaconda.ApiError. Got error message instead:", err.Error(), err)
			fs.Fatal(err.Error())
		} else {
			// These are usually transient eventual consistency errors (twitter breaks sometimes)
			for _, twitterError := range e.Decoded.Errors {
				switch code := twitterError.Code; code {
				case anaconda.TwitterErrorStatusIsADuplicate:
					log.Println("Non-fatal. Recovering from error:", e.Error())
					return true
				case anaconda.TwitterErrorDoesNotExist:
					log.Println("Non-fatal. Recovering from error:", e.Error())
					return true
				default:
					log.Println("Fatal error encountered.")
					fs.Fatal(err.Error())
				}
			}
		}

	}
	return true
}

// check has basic error handling and explains the error in a friendlier
// fashion when something goes poorly and the app crashes.
// The interface allows us to plug a different fatal call that doesn't crash
// in testing
func check(fs ErrorInterface, explain string, e error) {

	if e != nil {

		s := []string{explain, e.Error()}
		fs.Fatal(strings.Join(s, "\nMSG: "))
	}

}

// ConfigureApp brings up the configuration necessary to run the bot
func ConfigureApp(errorType ErrorInterface) {

	var configPath string

	flag.StringVar(&configPath, "c", "./config.json", "Configuration file, JSONized")
	flag.Parse()

	// logger setup
	// log.SetFlags(log.Lmicroseconds)
	log.Printf("Chim initializing. Built against git commit version: %v\n", gitCommit)

	// Start configuration loading
	log.Println("Loading application configuration from configuration file.")
	jsonData, err := ioutil.ReadFile(configPath)
	check(errorType, "Please create a config.json file, or set -c to a valid configuration file (see README.md for more details)", err)

	err = json.Unmarshal(jsonData, &config)
	check(errorType, "Couldn't unmarshal JSON from configuration file. Invalid syntax?", err)

	if config.ConsumerKey == "" || config.ConsumerSecret == "" || config.AccessToken == "" || config.AccessTokenSecret == "" {
		log.Fatal("At least one API credential is empty? Check JSON configuration file.")
	}

	// Configure Logrus
	// "Logrus has six logging levels: Debug, Info, Warning, Error, Fatal and Panic."
	switch level := config.LogrusLevel; level {
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "info":
		log.SetLevel(log.InfoLevel)
	case "warning":
		log.SetLevel(log.WarnLevel)
	case "error":
		log.SetLevel(log.ErrorLevel)
	case "fatal":
		log.SetLevel(log.FatalLevel)
	case "panic":
		log.SetLevel(log.PanicLevel)
	default:
		log.SetLevel(log.DebugLevel)
	}

	log.SetFormatter(&log.TextFormatter{
		ForceColors:            true,
		DisableLevelTruncation: true,
	})

	// Configure Prometheus metrics
	prometheus.MustRegister(tweetsProcessed)

	// Initialize twitter API
	anaconda.SetConsumerKey(config.ConsumerKey)
	anaconda.SetConsumerSecret(config.ConsumerSecret)
	api = anaconda.NewTwitterApi(config.AccessToken, config.AccessTokenSecret)

	// Token bucket args: token limit in seconds, 5 bufsize
	// TODO: Make this a configurable option
	api.EnableThrottling(3*time.Second, 5)

	// Initialize LRUs -- for longer description, see initial declarations

	// LRU: Check to see if a user is following a target
	tweetOriginatorLRU = lru.New(128)

	// LRU: Keep deltas for posts
	userPostDeltaLRU = lru.New(128)

	// LRU: Small LRU for keeping the last few posts we've seen so far
	postTextLRU = lru.New(25)

	// LRU: Very small LRU for keeping recent URLs we've posted
	urlLRU = lru.New(10)

	// Load gated content types into a memberset
	for _, types := range config.Settings.DeltaGatedContent {
		deltaGatedContent.Add(types)
	}

	// Add prohibited* to membersets
	for _, entries := range config.Settings.ProhibitedMentions {
		prohibitedMentions.Add(entries)
	}

	for _, entries := range config.Settings.ProhibitedWords {
		prohibitedWords.Add(entries)
	}

}

// .Retweet and .GetUsersLookup interfaces for production
type ApiInterface interface {
	Retweet(id int64, trimUser bool) (rt anaconda.Tweet, err error)
	GetUsersLookup(usernames string, v url.Values) (u []anaconda.User, err error)
}

type ApiAccess struct{}

func (fs ApiAccess) Retweet(id int64, trimUser bool) (rt anaconda.Tweet, err error) {
	return api.Retweet(id, trimUser)
}

func (fs ApiAccess) GetUsersLookup(usernames string, v url.Values) (u []anaconda.User, err error) {
	return api.GetUsersLookup(usernames, v)
}

// processTweet runs through validation and
// other steps before actually retweeting. Intended to be
// called via goroutine so we can do many re-tweets under
// processing load
func processTweet(a ApiInterface, fs FriendshipStatus, status anaconda.Tweet) bool {
	tweetLog := log.WithFields(log.Fields{"statusId": status.Id, "statusText": status.Text})

	tweetsProcessed.WithLabelValues("tweetsSeen", "count").Add(1)

	approved, tweetType, tweetContent := checkTweetContent(status)

	if !approved {
		tweetsProcessed.WithLabelValues("checkTweetContentReject", "reject").Add(1)
		return false
	} else {
		log.Printf("type: %v | content: %v | filter_level: %v", tweetType, tweetContent, status.FilterLevel)
	}

	// Check prohibited mention(s) for this tweet
	if checkForProhibitedMentions(status, prohibitedMentions) == false {
		tweetsProcessed.WithLabelValues("prohibitedMentions", "reject").Add(1)
		return false
	}

	if checkForProhibitedWords(status, prohibitedWords) == false {
		tweetsProcessed.WithLabelValues("prohibitedWords", "reject").Add(1)
		return false
	}

	// Check account age
	if checkAccountAge(status, config.Settings.MinAccountAgeHours) == false {
		tweetsProcessed.WithLabelValues("accountAgeHours", "reject").Add(1)
		return false
	}

	// Sleepy developer: Note the reversal of passing here.
	if userIsMuted(status.User.Id, mutedIds) == true {
		tweetsProcessed.WithLabelValues("mutedUserId", "reject").Add(1)
		return false
	}

	// Reject if the user posts certain kinds of content too quickly
	if checkContentDelta(status.User.Id, status.User.ScreenName, tweetType, deltaGatedContent, config.Settings.ContentTimeDelta, &status) == false {
		tweetsProcessed.WithLabelValues("contentTimeDelta", "reject").Add(1)
		return false
	}

	// Timing control for all posts we see from a user
	// Only status.User.Id is used for validation (its presumably static).
	// The ScreenName is used for debugging/display purposes (can vary).
	if checkUserPostDelta(status.User.Id, status.User.ScreenName, config.Settings.PostTimeDelta, &status) == false {
		tweetsProcessed.WithLabelValues("userPostDelta", "reject").Add(1)
		return false
	}

	// Have we seen the same post text recently? Happens with eventual-consistency sometimes.
	if checkPostRecentLRU(status.Text) == false {
		tweetsProcessed.WithLabelValues("postDuplicateInLRU", "reject").Add(1)
		return false
	}

	// Ensure user is following a target if we set MustFollow
	if checkUserFollowing(fs, status, config.Settings.MustFollow) == false {
		tweetsProcessed.WithLabelValues("mustFollow", "reject").Add(1)
		return false
	}

	// Decide what kind of action to take based on detected content
	// if any pre-filtering is required (such as content conversion)
	// this would be the spot to do it
	// -- example --
	// 	case tweetType == "video_clip":
	// 		uploadResult, err := a.UploadMedia(b64clip.String())
	// 		check(ErrorsAreFatal{}, "Could not upload media to twitter", err)
	// 		v := url.Values{}
	// 		v.Set("media_ids", uploadResult.MediaIDString)
	// 		// This needs to add the original tweet to the ending so that
	// 		// it turns into a quoted tweet
	// 		//result, err := api.PostTweet(string, v)
	// 		check(ErrorsAreFatal{}, "Could not post tweet", err)
	// 		log.Println("******************* QUOTED ************************")
	// 		log.Println("Posting new tweet with clip as quote")

	switch {
	default:
		tweetLog.Info("Retweeting")
		if config.TestMode == false {
			_, err := a.Retweet(status.Id, true)
			// Not expecting any errors, but in any scenario it is
			// likely to repeat/not good. Try to crash out.
			checkRetweetErrors(ErrorsAreFatal{}, "Could not retweet", err)
			tweetsProcessed.WithLabelValues("retweeted", "allow").Add(1)
		} else {
			tweetLog.Warn("Test mode; this tweet has not been retweeted because test_mode is true in the configuration")
		}
	}

	return true

}

// buildSearchTerms configures strings to be sent to the Twitter API
func buildSearchTerms(a ApiInterface, searchTerms string, watchUsers string) url.Values {
	values := url.Values{}

	values.Set("stall_warnings", "true")

	if config.Settings.TwitterFilterLevel != "" {
		log.Printf("Filter level set to: %v\n", config.Settings.TwitterFilterLevel)
		values.Set("filter_level", config.Settings.TwitterFilterLevel)
	}

	if len(searchTerms) > 0 {
		log.Printf(" ---> Search terms: %v\n", searchTerms)
		values.Set("track", searchTerms)
	}

	if len(watchUsers) > 0 {
		var userIDs []string
		users, err := a.GetUsersLookup(watchUsers, nil)

		check(ErrorsAreFatal{}, "Unable to convert screen names to twitter IDs", err)

		for _, u := range users {
			userIDs = append(userIDs, strconv.FormatInt(u.Id, 10))
		}
		userIDsToWatch := strings.Join(userIDs, ",")
		log.Printf(" ---> Watching for tweets from: [ users: %v ][ ids: %v ] \n", watchUsers, userIDsToWatch)
		values.Set("follow", userIDsToWatch)
	}

	return values
}

func runPublicStreamFilter() {
	log.Println("Started listening for events ..")
	log.Println(" ************ PublicStreamFilter ************ ")

	stream := api.PublicStreamFilter(buildSearchTerms(ApiAccess{}, config.SearchTerms, config.WatchUsers))

	// Enter listening loop. Use select to wait on multiple channels.
	for {
		select {
		case item := <-stream.C:
			switch status := item.(type) {
			case anaconda.Tweet:
				// Drop into a goroutine for this tweet and move onto the next tweet
				go processTweet(ApiAccess{}, FriendshipInfo{}, status)
				log.WithFields(log.Fields{
					"screenName": status.User.ScreenName,
					"text":       status.Text}).Debug("Processing")
				tweetsProcessed.WithLabelValues("total", "").Add(1)
			case anaconda.StallWarning:
				log.Warn("[WARNING] Processing latency. Queue at remote Twitter sender is full: ", status.PercentFull, "%")
			}
		}
	}
}

func main() {

	ConfigureApp(ErrorsAreFatal{})
	populateMutedList(MutedInfo{}, url.Values{}, mutedIds)

	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Info("This bot provides prometheus metrics. Available at http://127.0.0.1:8080/metrics")
		log.Fatal(http.ListenAndServe("127.0.0.1:8080", nil))
	}()

	runPublicStreamFilter()

}
