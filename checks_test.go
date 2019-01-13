package main

import (
	"github.com/ChimeraCoder/anaconda"
	"github.com/davidk/memberset"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestCalculateTweetTime(t *testing.T) {
	// Checking for golang API time consistency
	var testTimeCalculations = []struct {
		TestInfo          string
		Status            *anaconda.Tweet
		DeltaSeconds      int
		OutputCurrentTime time.Time
		durationTime      time.Duration
	}{
		{"Ensure that times are calculated properly",
			&anaconda.Tweet{
				CreatedAt: time.Now().Add(time.Duration(-24) * time.Hour).Format(time.RubyDate),
				User: anaconda.User{
					ScreenName: "chump",
				},
			},
			0,
			time.Now().Add(time.Duration(-24) * time.Hour).Truncate(time.Second),
			time.Duration(0),
		},
	}

	for _, testInput := range testTimeCalculations {
		currentTimeResult, deltaResult := calculateTweetTime(testInput.Status, testInput.DeltaSeconds)
		if currentTimeResult != testInput.OutputCurrentTime {
			t.Error(
				"Tried: ", testInput.TestInfo,
				"wanted: ", testInput.OutputCurrentTime,
				"got: ", currentTimeResult,
			)
		}

		if deltaResult != testInput.durationTime {
			t.Error(
				"Tried: ", testInput.TestInfo,
				"wanted: ", testInput.durationTime,
				"got: ", deltaResult,
			)
		}

	}
}

func TestCheckContentDelta(t *testing.T) {
	gatedContent := memberset.New()

	var testContentDelta = []struct {
		TestInfo         string
		User             int64
		Username         string
		ContentType      string
		GatedContent     *memberset.MemberSet
		TimeDeltaSeconds int
		Status           *anaconda.Tweet
		Expected         bool
	}{
		{"Add a piece of content. Should pass.",
			1337,
			"zoink",
			"bad",
			gatedContent,
			300,
			&anaconda.Tweet{
				CreatedAt: time.Now().Add(time.Duration(5) * time.Minute).Truncate(time.Second).Format(time.RubyDate),
			},
			true,
		},
		{"Try adding a similar piece of content. Should fail.",
			1337,
			"zoink",
			"bad",
			gatedContent,
			600,
			&anaconda.Tweet{
				CreatedAt: time.Now().Add(time.Duration(5) * time.Minute).Truncate(time.Second).Format(time.RubyDate),
			},
			false,
		},
		{"Try adding a different piece of content, from the same user.",
			1337,
			"zoink",
			"gif",
			gatedContent,
			10,
			&anaconda.Tweet{
				CreatedAt: time.Now().Add(time.Duration(5) * time.Minute).Truncate(time.Second).Format(time.RubyDate),
			},
			true,
		},
		{"With a deltatime longer than their last tweet: The user should be able to add more content",
			1337,
			"zoink",
			"gif",
			gatedContent,
			10,
			&anaconda.Tweet{
				CreatedAt: time.Now().Add((time.Duration(5) * time.Minute) + (time.Duration(11) * time.Second)).Truncate(time.Second).Format(time.RubyDate),
			},
			true,
		},
		{"Same user, completely different class of content (fuzzing)",
			1337,
			"zoink",
			"video",
			gatedContent,
			10,
			&anaconda.Tweet{
				CreatedAt: time.Now().Add(time.Duration(5) * time.Minute).Truncate(time.Second).Format(time.RubyDate),
			},
			true,
		},
		{"Try adding a different piece of content, from the same user.",
			1336,
			"Ryan",
			"video",
			gatedContent,
			10,
			&anaconda.Tweet{
				CreatedAt: time.Now().Add(time.Duration(5) * time.Minute).Truncate(time.Second).Format(time.RubyDate),
			},
			true,
		},
	}

	for _, testInput := range testContentDelta {
		result := checkContentDelta(testInput.User, testInput.Username, testInput.ContentType, testInput.GatedContent, testInput.TimeDeltaSeconds, testInput.Status)

		if result != testInput.Expected {
			t.Error(
				"Tried: ", testInput.TestInfo,
				"wanted: ", testInput.Expected,
				testInput.User,
				testInput.Username,
				testInput.ContentType,
				testInput.GatedContent,
				testInput.TimeDeltaSeconds,
				testInput.Status,
				"got: ", result,
			)
		}
	}
}

func TestCheckAccountAge(t *testing.T) {

	var testAccountAge = []struct {
		TestInfo string
		Status   anaconda.Tweet
		MinAge   int
		Output   bool
	}{
		{"Magnify rounding errors if present.",
			anaconda.Tweet{
				User: anaconda.User{
					ScreenName: "chump",
					CreatedAt:  "Wed Aug 27 13:08:45 +0000 2008",
				},
			}, 2871, true,
		},
		{"Allow 1 day old accounts. Normally un-checkable due to sub-second precision, but hours are rounded out",
			anaconda.Tweet{
				User: anaconda.User{
					ScreenName: "porter",
					CreatedAt:  time.Now().Add(-time.Duration(24) * time.Hour).Format(time.RubyDate),
				},
			}, 1, false,
		},
		{"Deny 1 day in the future returned from twitter API. Fails.",
			anaconda.Tweet{
				User: anaconda.User{
					ScreenName: "chell",
					CreatedAt:  time.Now().Add(time.Duration(24) * time.Hour).Format(time.RubyDate),
				},
			}, 1, false},
		{"Allow accounts created 0 days ago, or just now",
			anaconda.Tweet{
				User: anaconda.User{
					ScreenName: "bank",
					CreatedAt:  "Wed Aug 27 13:08:45 +0000 2008"},
			}, 0, true},
		{"Allow checks for accounts made in the future. Should clamp to 0, and return success.",
			anaconda.Tweet{
				User: anaconda.User{
					ScreenName: "spy",
					CreatedAt:  "Wed Aug 27 13:08:45 +0000 2008"},
			}, -5, true},
	}

	for _, testInput := range testAccountAge {
		result := checkAccountAge(testInput.Status, testInput.MinAge)
		if result != testInput.Output {
			t.Error(
				"Tried: ", testInput.TestInfo,
				"wanted: ", testInput.Output,
				"Got: ", result,
			)
		}
	}

}

// TestCheckUserPostDelta checks checkUserPostDelta in order to verify
// whether or not we properly clamp users who may post too much in a
// certain time period
func TestCheckUserPostDelta(t *testing.T) {

	// This is a dirty test trick, but the cache is designed to stop
	// quickly repeating spam posts from many different users,
	// so hypothetically we can just set a Duration of 0 and execute
	// sequentially to fill the LRU/test as necessary.
	var testsPostDeltaLRU = []struct {
		User               int64
		Username           string
		Output             bool
		DelayBeforeSending time.Duration
		Tweet              anaconda.Tweet
	}{
		{
			728684316415295488,
			"chim",
			true,
			time.Duration(0),
			anaconda.Tweet{CreatedAt: "Wed Aug 27 13:08:45 +0000 2008"},
		},
		{
			728684316415295488,
			"chim",
			false,
			time.Duration(0),
			anaconda.Tweet{CreatedAt: "Wed Aug 27 13:08:45 +0000 2008"},
		},
		{
			728684316415295489,
			"unknown",
			true,
			time.Duration(0),
			anaconda.Tweet{CreatedAt: "Wed Aug 27 13:08:45 +0000 2008"},
		},
		{
			728684316415295489,
			"unknown",
			true,
			time.Duration(0),
			anaconda.Tweet{CreatedAt: "Wed Aug 27 13:08:47 +0000 2008"},
		},
		{
			728684316415295487,
			"unknown",
			true,
			time.Duration(0),
			anaconda.Tweet{CreatedAt: "Wed Aug 27 13:08:45 +0000 2008"},
		},
	}

	for _, testInput := range testsPostDeltaLRU {
		time.Sleep(time.Second * testInput.DelayBeforeSending)
		result := checkUserPostDelta(testInput.User, testInput.Username, 2, &testInput.Tweet)
		if result != testInput.Output {
			t.Error(
				"Tried: ", testInput.User,
				"Username: ", testInput.Username,
				"wanted: ", testInput.Output,
				"Got: ", result,
			)
		}
	}

}

func TestCheckTweetContent(t *testing.T) {

	config = AppConfiguration{
		Settings: InternalTuning{
			IgnoreFrom:           "chim",
			MustFollow:           "jack",
			DenySensitiveContent: true,
			MinAccountAgeHours:   5,
		},
	}

	var testscheckTweetContent = []struct {
		TestInfo string
		Input    anaconda.Tweet
		Output   bool
	}{
		{"Deny if tweet is possibly sensitive",
			anaconda.Tweet{
				Text:              "This is a really sensitive comment. Rawar.",
				PossiblySensitive: true,
			}, false,
		},
		{"Allow when tweet is not sensitive",
			anaconda.Tweet{
				Text:              "This is a really insensitive comment. Meep.",
				PossiblySensitive: false,
				Entities: anaconda.Entities{
					// Dev note: We have to redefine the struct since it isn't
					// named. See golang-nuts here:
					// http://comments.gmane.org/gmane.comp.lang.go.general/95405
					Urls: []struct {
						Indices      []int  "json:\"indices\""
						Url          string "json:\"url\""
						Display_url  string "json:\"display_url\""
						Expanded_url string "json:\"expanded_url\""
					}{
						{Expanded_url: "https://example.com/"},
					},
					Media: []anaconda.EntityMedia{
						{Type: "animated_gif",
							VideoInfo: anaconda.VideoInfo{
								Variants: []anaconda.Variant{
									{Url: "example.com"},
								},
							},
						},
					},
				},
			}, true,
		},
		{"Fail a tweet with all the right conditions set",
			anaconda.Tweet{
				InReplyToScreenName: "chim",
				RetweetedStatus:     nil,
				Text:                "RT I'm a retweetable tweet.",
				User:                anaconda.User{ScreenName: "chim"},
			}, false,
		},
		{"InReplyToScreenName should be empty",
			anaconda.Tweet{
				InReplyToScreenName: "yes"}, false},
		{"RetweetedStatus should be nil",
			anaconda.Tweet{
				RetweetedStatus: &anaconda.Tweet{}}, false},
		{"Check that tweet does not have RT prefix",
			anaconda.Tweet{
				Text: "RT Learn his one secret!"}, false},
		{"Ignore tweets from a single user in configuration",
			anaconda.Tweet{
				User: anaconda.User{ScreenName: "chim"}}, false},
		{"Deny empty expanded URL case",
			anaconda.Tweet{
				User: anaconda.User{ScreenName: "fluffy"},
				Text: "DYK we have killer bunnies? They are extremely fluffy!",
				Entities: anaconda.Entities{
					// Dev note: We have to redefine the struct since it isn't
					// named. See golang-nuts here:
					// http://comments.gmane.org/gmane.comp.lang.go.general/95405
					Urls: []struct {
						Indices      []int  "json:\"indices\""
						Url          string "json:\"url\""
						Display_url  string "json:\"display_url\""
						Expanded_url string "json:\"expanded_url\""
					}{{Expanded_url: "https://google.com"}},
				},
			}, // end anaconda.Tweet
			false,
		},
		{"Support animated_gif media type",
			anaconda.Tweet{
				User: anaconda.User{ScreenName: "doctor_fluffy"},
				Text: "Its anime day!",
				ExtendedEntities: anaconda.Entities{
					Media: []anaconda.EntityMedia{
						{Type: "animated_gif",
							VideoInfo: anaconda.VideoInfo{
								Variants: []anaconda.Variant{
									{Url: "example.com"},
								},
							},
						},
					},
				},
			}, // end anaconda.Tweet
			true,
		},
		{"Should support video media type",
			anaconda.Tweet{
				User: anaconda.User{ScreenName: "doctor_fluffy"},
				Text: "Its bad anime day!",
				ExtendedEntities: anaconda.Entities{
					Media: []anaconda.EntityMedia{
						{
							Type: "video",
							VideoInfo: anaconda.VideoInfo{
								Variants: []anaconda.Variant{
									{Url: "example.com"},
								},
							},
						},
					},
				},
			}, // end anaconda.Tweet
			true,
		},
	}

	for _, testInput := range testscheckTweetContent {
		result, _, _ := checkTweetContent(testInput.Input)
		if result != testInput.Output {
			t.Error(
				"Tried: ", testInput.TestInfo,
				"wanted: ", testInput.Output,
				"Got: ", result,
			)
		}
	}
}

// TestCheckPostRecentLRU tests checkPostRecentLRU to ensure
// that the LRU properly returns for duplicate postings
func TestCheckPostRecentLRU(t *testing.T) {

	var testsPostLRU = []struct {
		Input  string
		Output bool
	}{
		{"So cool that they cant touch this", true},
		{"So cool that they cant touch this", false},
	}

	for _, testInput := range testsPostLRU {
		result := checkPostRecentLRU(testInput.Input)
		if result != testInput.Output {
			t.Error(
				"Tried: ", testInput.Input,
				"wanted: ", testInput.Output,
				"Got: ", result,
			)
		}
	}

}

// TestCheckUserFollowing tests checkUserFollowing to ensure that a tweet's
// origin user actually follows a certain target
func TestCheckUserFollowing(t *testing.T) {

	var testFollowingTweet = []struct {
		TestInfo   string
		Input      anaconda.Tweet
		MustFollow string
		Output     bool
	}{
		{
			"Allow tweets that originate from the target (user doesn't really follow itself)",
			anaconda.Tweet{
				User: anaconda.User{ScreenName: config.Settings.MustFollow},
				Text: "See how our GM ate the entire group!",
			},
			config.Settings.MustFollow,
			true,
		},
		{
			"Accept: Mutual follow",
			anaconda.Tweet{
				User: anaconda.User{ScreenName: "bothFollow"},
				Text: "See how our PM ate the entire group!",
			},
			"bothFollow",
			true,
		},
		{
			"Deny: Mutual mode on, source follows but target does not",
			anaconda.Tweet{
				User: anaconda.User{ScreenName: "sourceFollowsMutualModeOn"},
				Text: "Stop SPAM dead today, with DUCKS THAT EAT IT ALL",
			},
			"mutual_mode_failure",
			false,
		},
		{
			"Accept: Mutual mode on, source and target follow each other",
			anaconda.Tweet{
				User: anaconda.User{ScreenName: "mutual_mode"},
				Text: "Stop SPAM dead today, with WABBIT TWAPS",
			},
			"both_follow_enforced",
			true,
		},
		{
			"Accept: source follows, but target does not",
			anaconda.Tweet{
				User: anaconda.User{ScreenName: "sourceFollows"},
				Text: "ALWAYS CLICK BUNNIES",
			},
			"target",
			true,
		},
		{
			"Retrieve from cache: source follows, but target does not",
			anaconda.Tweet{
				User: anaconda.User{ScreenName: "sourceFollows"},
				Text: "NEVER CLICK BUNNIES",
			},
			"target",
			true,
		},
		{
			"Deny: source follows, but target does not",
			anaconda.Tweet{
				User: anaconda.User{ScreenName: "noFollow"},
				Text: "NEVER CLICK ON UNICORNS",
			},
			"target",
			false,
		},
		{
			"Deny from cache: source follows, but target does not",
			anaconda.Tweet{
				User: anaconda.User{ScreenName: "noFollow"},
				Text: "ALWAYS CLICK ON UNICORNS",
			},
			"target",
			false,
		},
		{
			"Allow: Source and target do not follow each other",
			anaconda.Tweet{
				User: anaconda.User{ScreenName: "noSourceTargetFollow"},
				Text: "SCIENCE IS LOGIC BUT BOTH MUST HAVE HEART",
			},
			"",
			true,
		},
		{
			"Deny: Target follows a user, but source does not. The target may be interested in user, but user likely tweets about a lot of non-relevant things if not following target directly. May be changed in the future.",
			anaconda.Tweet{
				User: anaconda.User{ScreenName: "source"},
				Text: "SCIENCE IS LOGIC BUT BOTH MUST HAVE HEART",
			},
			"targetFollows",
			false,
		}, // ..
	}

	for _, testInput := range testFollowingTweet {

		if testInput.MustFollow == "mutual_mode_failure" || testInput.MustFollow == "both_follow_enforced" {
			config.Settings.MutualFollow = true
		} else {
			config.Settings.MutualFollow = false
		}

		result := checkUserFollowing(FakeFriendshipInfo{}, testInput.Input, testInput.MustFollow)
		if result != testInput.Output {
			t.Errorf("Tried: %v\nWanted: %v\nGot: %v -- mutual mode: %t\n", testInput.TestInfo, testInput.Output, result, config.Settings.MutualFollow)
		}
	}

}

// TestCheckDuplicateContent tests the checkDuplicateContent
// LRU to ensure that it functions with a size of 5 for
// media content URLs that we retrieve
func TestCheckDuplicateContent(t *testing.T) {
	var testDuplicateContent = []struct {
		Input  string
		Output bool
	}{
		{Input: "https://google.com", Output: true},
		{Input: "https://google.com", Output: false},
		{Input: "https://altavista.com", Output: true},
		{Input: "https://periscope.tv/w/", Output: true},
		{Input: "https://google.com", Output: false},
	}

	for _, testInput := range testDuplicateContent {
		result := checkDuplicateContent(testInput.Input)
		if result != testInput.Output {
			t.Errorf("Tried %v, but got %v.", testInput.Input, testInput.Output)
		}
	}

}

// TestUserIsMuted exercises memberset's setters and getters, with varying types.
// We rely on memberset's behavior being consistent, so it needs to be tested.
// Note: The failing case is also tested: a key that does not exist and is
// retrieved.
func TestUserIsMuted(t *testing.T) {
	testIds := memberset.New()

	testIds.Add(1234)
	testIds.Add(int64(12345))
	testIds.Add(string(8888))

	if ok := userIsMuted(8888, testIds); ok {
		t.Error("Able to retrieve non-matching ID with int type")
	}

	if ok := userIsMuted(string(8888), testIds); !ok {
		t.Error("Unable to retrieve ID with matching type")
	}

	if ok := userIsMuted(1234, testIds); !ok {
		t.Error("Unable to retrieve muted int UID 1234 placed in set.")
	}

	if ok := userIsMuted(int64(12345), testIds); !ok {
		t.Error("Unable to retrieve muted int64 UID 12345 placed in set.")
	}

	if ok := userIsMuted(0000, testIds); ok {
		t.Error("Muted non-existent user not placed in set.")
	}

	if ok := userIsMuted(int64(0000), testIds); ok {
		t.Error("Muted non-existent user not placed in set.")
	}

}

// FakeFriendshipInfo is passed to checkUserFollowing to 'stub/mock'
// out the API call to twitter
type FakeFriendshipInfo struct {
	Response anaconda.RelationshipResponse
	Err      error
}

// GetFriendshipStatus does some magical things for a test call
//    SEND THIS  = GET THIS BACK
//    --------------------------
// .. Set targetName = "targetFollows"
// RelationshipResponse Eq: target: true, following: false
//
// .. Set sourceName = sourceFollows
// RelationshipResponse Eq: target: false, following: true
//
// .. Set sourceName = bothFollow or  Set targetName = bothFollow
// RelationshipResponse Eq: target: true, following: true
func (f FakeFriendshipInfo) GetFriendshipStatus(v url.Values) (anaconda.RelationshipResponse, error) {
	if f.Err != nil {
		return anaconda.RelationshipResponse{}, f.Err
	}

	sourceName := v.Get("source_screen_name")
	targetName := v.Get("target_screen_name")

	switch {
	// Source of tweet follows
	case strings.EqualFold(sourceName, "sourceFollows"):
		return anaconda.RelationshipResponse{
			anaconda.Relationship{
				anaconda.Target{
					Following: false},
				anaconda.Source{
					Following: true},
			},
		}, nil
	// Target user in configuration follows, but source does not
	case strings.EqualFold(targetName, "targetFollows"):
		return anaconda.RelationshipResponse{
			anaconda.Relationship{
				anaconda.Target{
					Following: true},
				anaconda.Source{
					Following: false},
			},
		}, nil
	// Target user and source follow each other
	case strings.EqualFold(targetName, "bothFollow") || strings.EqualFold(sourceName, "bothFollow"):
		return anaconda.RelationshipResponse{
			anaconda.Relationship{
				anaconda.Target{
					Following: true},
				anaconda.Source{
					Following: true},
			},
		}, nil
	// Source follows, target does not, but we are in mutual mode and not one-way mode
	case strings.EqualFold(sourceName, "sourceFollowsMutualModeOn"):
		return anaconda.RelationshipResponse{
			anaconda.Relationship{
				anaconda.Target{
					Following: false},
				anaconda.Source{
					Following: true},
			},
		}, nil
	// Source and target follow each other. Mutual mode is considered to be on.
	case strings.EqualFold(sourceName, "mutual_mode"):
		return anaconda.RelationshipResponse{
			anaconda.Relationship{
				anaconda.Target{
					Following: true},
				anaconda.Source{
					Following: true},
			},
		}, nil
	case strings.EqualFold(sourceName, "noSourceTargetFollow"):
		return anaconda.RelationshipResponse{
			anaconda.Relationship{
				anaconda.Target{
					Following: false},
				anaconda.Source{
					Following: false},
			},
		}, nil
	// Not friends at all
	default:
		return anaconda.RelationshipResponse{
			anaconda.Relationship{
				anaconda.Target{
					Following: false},
				anaconda.Source{
					Following: false},
			},
		}, nil
	}
}
