package main

import (
	"errors"
	"github.com/ChimeraCoder/anaconda"
	"github.com/davidk/lru"
	"github.com/davidk/memberset"
	"net/url"
	"strings"
	"testing"
)

func init() {
	// Testing LRUs
	tweetOriginatorLRU = lru.New(128)
	userPostDeltaLRU = lru.New(128)
	// Small LRU for keeping the last few posts we've seen so far
	postTextLRU = lru.New(25)
	urlLRU = lru.New(5)
}

func printDebug(t *testing.T) {

	t.Log("*** Configuration details ***")
	t.Logf("ConsumerKey: %v, ConsumerSecret: %v, AccessToken: %v, AccessTokenSecret: %v, SearchTerms: %v, WatchUsers: %v", config.ConsumerKey, config.ConsumerSecret, config.AccessToken, config.AccessTokenSecret, config.SearchTerms, config.WatchUsers)

}

// TestCheck
type FakeFatal struct{}

func (fs FakeFatal) Fatal(format string, v ...interface{}) {}

// TestCheck tests error handling in main
func TestCheck(t *testing.T) {
	check(FakeFatal{}, "Explain the error", nil)
	check(FakeFatal{}, "TestCheck PASS", errors.New("PASSED"))
}

func TestCheckRetweetErrors(t *testing.T) {

	passingError := anaconda.ApiError{
		Decoded: anaconda.TwitterErrorResponse{
			Errors: []anaconda.TwitterError{
				{
					Message: "This is an error message and it is expected to pass",
					Code:    anaconda.TwitterErrorStatusIsADuplicate,
				},
			},
		},
	}

	failingError := anaconda.ApiError{
		Decoded: anaconda.TwitterErrorResponse{
			Errors: []anaconda.TwitterError{
				{
					Message: "This is an error message and it is expected to fail",
					Code:    anaconda.TwitterErrorCouldNotAuthenticate,
				},
			},
		},
	}

	fatal := checkRetweetErrors(FakeFatal{}, "This is a passing error", nil)
	if !fatal {
		t.Error("checkRetweetErrors: Test for FakeFatal stub did not pass.")
	}

	fatal = checkRetweetErrors(FakeFatal{}, "Handled error, duplicate tweet", &passingError)
	if !fatal {
		t.Error("checkRetweetErrors: Handled error crashed.")
	}

	fatal = checkRetweetErrors(FakeFatal{}, "Unhandled error, we should crash", &failingError)
	if !fatal {
		t.Error("checkRetweetErrors: Unhandled error was not fatal? (We did not crash)")
	}

}

// for processTweet
type FakeApiRetweet struct {
	Response anaconda.Tweet
	Error    error
}

// for processTweet
func (fs FakeApiRetweet) Retweet(id int64, trimUser bool) (rt anaconda.Tweet, err error) {
	return anaconda.Tweet{}, nil
}

func (fs FakeApiRetweet) GetUsersLookup(usernames string, v url.Values) (u []anaconda.User, err error) {
	return []anaconda.User{{Id: 12345}, {Id: 6789}, {Id: 101112131415}}, nil
}

// FakeMuteInfo is passed to GetMutedUsersList to stub/mock
// the call to twitter
type FakeMuteInfo struct {
	Response anaconda.UserCursor
	Err      error
}

// GetMutedUsersList contains the fake response we send
func (fs FakeMuteInfo) GetMutedUsersList(v url.Values) (anaconda.UserCursor, error) {

	// If we get a cursor to dig deeper into results,
	// return a simulated "next page"
	if ok := v.Get("cursor"); ok == "1" {
		return anaconda.UserCursor{
			Previous_cursor:     1,
			Previous_cursor_str: "1",
			Next_cursor:         2,
			Next_cursor_str:     "2",
			Users: []anaconda.User{
				{Id: 4567, ScreenName: "recurse_this"},
			},
		}, fs.Err
	}

	// Test at least 3 levels, so we can ensure that the
	// recurse call actually works properly
	if ok := v.Get("cursor"); ok == "2" {
		return anaconda.UserCursor{
			Previous_cursor:     2,
			Previous_cursor_str: "2",
			Next_cursor:         0,
			Next_cursor_str:     "0",
			Users: []anaconda.User{
				{Id: 8910, ScreenName: "recurse_this"},
			},
		}, fs.Err
	}

	// Default return. By default this should trigger the
	// above recursed return
	return anaconda.UserCursor{
		Previous_cursor:     0,
		Previous_cursor_str: "0",
		Next_cursor:         1,
		Next_cursor_str:     "1",
		Users: []anaconda.User{
			{Id: 1234, ScreenName: "cake"},
		},
	}, fs.Err

}

func TestConfigureApp(t *testing.T) {

	config.ConsumerKey = "a"
	config.ConsumerSecret = "b"
	config.AccessToken = "c"
	config.AccessTokenSecret = "d"

	ConfigureApp(FakeFatal{})

}

func TestBuildSearchTerms(t *testing.T) {
	// To cover filter_level
	config.Settings.TwitterFilterLevel = "cake"

	searchTerms := "cake,fluffy1,fluffy2,fluffy3,fluffy4"
	followTerms := "12345,6789,101112131415"

	values := buildSearchTerms(FakeApiRetweet{}, searchTerms, followTerms)

	if !strings.EqualFold(values.Get("track"), searchTerms) {
		t.Errorf("List of terms to track is not correct. %v == %v", searchTerms, values.Get("track"))
	}

	if !strings.EqualFold(values.Get("follow"), followTerms) {
		t.Errorf("List of users to follow not correct %v == %v", followTerms, values.Get("follow"))
	}
}

func TestProcessTweet(t *testing.T) {
	mutedIds = memberset.New()
	var testProcessTweet = []struct {
		Explain string
		Tweet   anaconda.Tweet
		Output  bool
	}{
		{Explain: "Fail checkTweetContent", Tweet: anaconda.Tweet{}, Output: false},
		{Explain: "Pass checkTweetContent",
			Tweet: anaconda.Tweet{
				CreatedAt:         "Wed Aug 27 13:08:45 +0000 2008",
				Text:              "I have cake!",
				PossiblySensitive: false,
				User: anaconda.User{
					Id:         1337821,
					ScreenName: "bothFollow",
					CreatedAt:  "Wed Aug 27 13:08:45 +0000 2008"},
				ExtendedEntities: anaconda.Entities{
					Media: []anaconda.EntityMedia{
						{Type: "animated_gif",
							VideoInfo: anaconda.VideoInfo{
								Variants: []anaconda.Variant{
									{Url: "http://example.com"},
								},
							},
						},
					},
				},
			}, Output: true,
		},
		{Explain: "Pass checkTweetContent with Video",
			Tweet: anaconda.Tweet{
				CreatedAt:         "Wed Aug 27 13:08:45 +0000 2008",
				Text:              "I have pie!",
				PossiblySensitive: false,
				User: anaconda.User{
					Id:         1337821,
					ScreenName: "bothFollow",
					CreatedAt:  "Wed Aug 27 13:08:45 +0000 2008"},
				ExtendedEntities: anaconda.Entities{
					Media: []anaconda.EntityMedia{
						{Type: "video",
							VideoInfo: anaconda.VideoInfo{
								Variants: []anaconda.Variant{
									{Url: "http://example.com"},
								},
							},
						},
					},
				},
			}, Output: true,
		},
	}

	for _, testInput := range testProcessTweet {
		result := processTweet(FakeApiRetweet{}, FakeFriendshipInfo{}, testInput.Tweet)
		if result != testInput.Output {
			t.Error(
				"Tried: ", testInput.Explain,
				"Wanted: ", testInput.Output,
				"Got: ", result,
			)
		}
	}
}

func TestPopulateMuteList(t *testing.T) {
	mutedIds = memberset.New()
	populateMutedList(FakeMuteInfo{}, url.Values{}, mutedIds)

	testIds := []int64{1234, 4567, 8910, 1111}

	if ok := mutedIds.Get(testIds[0]); !ok {
		t.Errorf("Unable to find ID 1234 in output")
	}

	if ok := mutedIds.Get(testIds[1]); !ok {
		t.Errorf("Unable to find recursed ID 4567 in output")
	}

	if ok := mutedIds.Get(testIds[2]); !ok {
		t.Errorf("Unable to find recursed ID 8910 in output")
	}

	mutedIds.Add(testIds[3])
	if ok := mutedIds.Get(testIds[3]); !ok {
		t.Errorf("Unable to find 1111 in output")
	}

	mutedIds.Delete(testIds[3])
	if ok := mutedIds.Get(testIds[3]); ok {
		t.Errorf("1111 was not deleted.")
	}

}
