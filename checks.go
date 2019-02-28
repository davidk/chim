// Checks/filters/rate-limits that are used for the bot to ensure
// that retweets are somewhat high quality
package main

import (
	"github.com/davidk/anaconda"
	"github.com/davidk/memberset"
	log "github.com/sirupsen/logrus"
	"net/url"
	"strings"
	"time"
)

// checkAccountAge checks the account age for a particular origin status/tweet.
// if the account is < the minAgeHours threshold, it returns false.
// If minAgeHours is < 0, the value is automatically clamped to 0.
func checkAccountAge(status anaconda.Tweet, minAgeHours int) bool {

	// Parse both into time fields
	// N.B.: RubyDate might be weirdly defined, and will probably(?) change
	// in the future.
	// https://github.com/golang/go/issues/518
	//
	// Ruby Time.now:
	// irb(main):001:0> Time.now
	// => 2016-07-04 01:26:37 -0700
	//
	// https://dev.twitter.com/overview/api/tweets
	// Twitter claims it returns the following:
	// "created_at":"Wed Aug 27 13:08:45 +0000 2008"
	// Twitter's created_at format is defined as RubyDate in Go

	if minAgeHours < 0 {
		minAgeHours = 0
	}

	minAgeDuration := time.Duration(minAgeHours*24) * time.Hour

	log.WithFields(log.Fields{
		"screenName":          status.User.ScreenName,
		"minAgeHoursRequired": minAgeHours,
		"userCreationDate":    status.User.CreatedAt,
	}).Debug("checkAccountAge: Checking account age")

	userCreatedAt, err := time.Parse(time.RubyDate, status.User.CreatedAt)
	check(ErrorsAreFatal{}, "checkAccountAge: Unable to parse time.", err)

	// Round everything to the nearest hour
	userCreatedAt = userCreatedAt.Truncate(time.Nanosecond)
	timeSinceUserCreated := time.Now().Truncate(time.Hour).Sub(userCreatedAt)

	log.WithFields(log.Fields{
		"accountAge": time.Duration(timeSinceUserCreated) / 24,
		"minAge":     time.Duration(minAgeDuration) / 24,
	}).Info("checkAccountAge")

	if timeSinceUserCreated > minAgeDuration {
		log.Info("checkAccountAge: Account is older than required hours. OK.")
		return true
	}

	log.Info("checkAccountAge: Account is NOT older than required hours. FAIL.")
	return false
}

// checkDuplicateContent checks a LRU to see if the URL given has already
// been posted in recent memory. If it is, we return false.
func checkDuplicateContent(url string) bool {

	if _, present := urlLRU.Get(url); !present {
		log.Infof("checkDuplicateContent: CACHE MISS - ACCEPT - Content does not exist in LRU: %v\n", url)
		urlLRU.Add(url, 1)
		return true
	}

	log.Infof("checkDuplicateContent: CACHE HIT - REJECT - Content exists in LRU: %v\n", url)
	return false
}

// checkTweetContent sees if the data is re-tweetable or not
// This is not a through check, since opening each image in a large stream
// to check it's magic bits is beyond the technical scope of this app ATM
// Return values are:
// approval       - boolean
// type of tweet  - string
func checkTweetContent(status anaconda.Tweet) (approved bool, tweetType string, contentURL string) {
	log.Debug("Incoming status text:", status.Text)
	// Tweet must be an original message, .RetweetedStatus is usually correct,
	// but an RT prefix is also prevalent ("manual retweeting")
	if len(status.InReplyToScreenName) != 0 ||
		status.RetweetedStatus != nil ||
		strings.HasPrefix("RT", status.Text) ||
		strings.EqualFold(status.User.ScreenName, config.Settings.IgnoreFrom) {

		log.Debug("checkTweetContent REJECT: Not a global original message detected.")

		return false, "non-original", ""
	}

	// Deny if tweet contains material that can be considered to be sensitive
	if config.Settings.DenySensitiveContent == true && status.PossiblySensitive == true {
		log.Debug("checkTweetContent REJECT: Sensitive content detected")
		return false, "sensitive", ""
	}

	if approved, tweetType, contentURL = checkEntityMedia(status, status.Entities.Media); approved {
		return
	}

	if approved, tweetType, contentURL = checkEntityMedia(status, status.ExtendedEntities.Media); approved {
		return
	}

	if approved, tweetType, contentURL = checkEntityMedia(status, status.ExtendedTweet.Entities.Media); approved {
		return
	}

	if approved, tweetType, contentURL = checkEntityMedia(status, status.ExtendedTweet.ExtendedEntities.Media); approved {
		return
	}

	log.Debug("checkTweetContent REJECT: No media content found")
	return false, "no_match", ""
}

func checkEntityMedia(status anaconda.Tweet, mediaEntries []anaconda.EntityMedia) (bool, string, string) {
	for _, media := range mediaEntries {
		switch {
		case strings.EqualFold(media.Type, "animated_gif"):
			log.Debugf("checkEntityMedia found %v: %v\n", status.User.ScreenName, status.Text)

			for _, mv := range media.VideoInfo.Variants {
				log.Debugf("checkEntityMedia found %v: %v\n", mv.ContentType, mv.Url)
			}

			log.Debugf("checkEntityMedia found %v: %v\n", media.Type, media.Media_url_https)
			return true, "gif", media.Media_url_https
		case strings.EqualFold(media.Type, "video"):
			log.Debugf("checkEntityMedia found %v: %v\n", status.User.ScreenName, status.Text)

			for _, mv := range media.VideoInfo.Variants {
				log.Debugf("checkEntityMedia found %v: %v\n", mv.ContentType, mv.Url)
			}

			log.Debugf("checkEntityMedia found %v: %v\n", media.Type, media.Media_url_https)
			return true, "video", media.Media_url_https
		}
	}
	return false, "", ""
}

// FriendshipStatus helps to wrap the Anaconda GetFriendshipShow for testing
type FriendshipStatus interface {
	GetFriendshipStatus(url.Values) (anaconda.RelationshipResponse, error)
}

// FriendshipInfo is replaced in testing with fake versions that
// do not call out to the Twitter API. A FriendshipInfo is passed to
// checkUserFollowing and calls api.GetFriendshipsShow by proxy
type FriendshipInfo struct{}

// GetFriendshipStatus will grab friendships from anaconda's API
func (fs FriendshipInfo) GetFriendshipStatus(v url.Values) (anaconda.RelationshipResponse, error) {
	return api.GetFriendshipsShow(v)
}

// checkUserFollowing, checks the user that originated the tweet.
// This performs some basic checking to ensure that they are not
// too abusive. Note, according to: https://dev.twitter.com/rest/reference/get/friendships/show
// this is rate-limited to a 15-minute window (app) / 180 user auth
func checkUserFollowing(fs FriendshipStatus, status anaconda.Tweet, targetUser string) bool {

	if strings.EqualFold(status.User.ScreenName, targetUser) {
		log.Infof("userIsFollowing: Tweet originated from our target. Bypassing check.")
		return true
	}

	if targetUser == "" {
		log.Infof("userIsFollowing: Setting must_follow is not set. Bypassing check for user: %v\n",
			status.User.ScreenName)
		return true
	}

	log.Infof("userIsFollowing: Checking origin twitter handle: %v. User must follow %v. [mutual mode: %t]\n",
		status.User.ScreenName, targetUser, config.Settings.MutualFollow)

	val, _ := tweetOriginatorLRU.Get(status.User.ScreenName)

	if val == 1 {
		log.Info("userIsFollowing: LRU cache hit.")
		return true
	} else if val == nil {
		log.Info("userIsFollowing: LRU cache miss.")
		// cache missed/no entry
	} else if val == 0 {
		log.Info("userIsFollowing: LRU cache hit.")
		return false
	}

	log.Println("userIsFollowing: Performing live check.")

	values := url.Values{}
	values.Set("source_screen_name", status.User.ScreenName)
	values.Set("target_screen_name", targetUser)

	// caches missed, do live query
	r, _ := fs.GetFriendshipStatus(values)

	log.Infof("userIsFollowing: Does [ %v follow %v ] ? %v [ does %v follow? %v ]\n", status.User.ScreenName,
		targetUser, r.Relationship.Source.Following, targetUser, r.Relationship.Target.Following)

	if r.Relationship.Source.Following && config.Settings.MutualFollow == false {

		log.Infof("userIsFollowing [tweet source follows, mutual %t]: %v is following target.\n",
			config.Settings.MutualFollow,
			status.User.ScreenName)

		tweetOriginatorLRU.Add(status.User.ScreenName, 1)

		log.Info("userIsFollowing [tweet source follows, mutual off]: added user to LRU cache (1). User is following.")

		return true

	} else if config.Settings.MutualFollow == true {

		if r.Relationship.Source.Following && r.Relationship.Target.Following {

			tweetOriginatorLRU.Add(status.User.ScreenName, 1)
			log.Info("userIsFollowing [mutual mode]: added user to LRU cache (1). User and target are following each other.")
			return true

		}

		tweetOriginatorLRU.Add(status.User.ScreenName, 0)
		log.Info("userIsFollowing [mutual mode]: Target/Source are not mutually following each other. Rejected.")
		return false

	} else {

		tweetOriginatorLRU.Add(status.User.ScreenName, 0)
		log.Info("userIsFollowing: LRU cache add (0). User not following.")
		return false

	}
}

// calculateTweetTime converts string based times and seconds to values
// that are easier to use with Go's time libraries
func calculateTweetTime(status *anaconda.Tweet, timeDeltaSeconds int) (time.Time, time.Duration) {

	createdTime, err := time.Parse(time.RubyDate, status.CreatedAt)
	check(ErrorsAreFatal{}, "calculateTweetTime: Unable to parse time.", err)

	createdTime = createdTime.Truncate(time.Second)

	// convert timeDeltaSeconds to a duration amount in seconds
	deltaDuration := time.Duration(timeDeltaSeconds) * time.Second

	return createdTime, deltaDuration
}

// ContentDelta contains the User and a ContentType for embedding in an LRU
type ContentDelta struct {
	User        int64
	ContentType string
}

// checkContentDelta delays a user posting the same kind of content
// I.E clips over and over. This is in addition to the "global" user
// delta.
func checkContentDelta(user int64, username string, contentType string,
	gatedContent *memberset.MemberSet, timeDeltaSeconds int, status *anaconda.Tweet) bool {
	log.Infof("checkContentDelta: Checking content delta for user %v. Content delta is set to %v. Content type: %v", user, timeDeltaSeconds, contentType)

	currentTime, deltaDuration := calculateTweetTime(status, timeDeltaSeconds)
	val, present := userContentDeltaLRU.Get(ContentDelta{user, contentType})

	if !present {
		log.Infof("checkContentDeltaLRU: CACHE MISS - ACCEPT - User %v not in content delta cache. Content type: %v\n", username, contentType)
		userContentDeltaLRU.Add(ContentDelta{user, contentType}, currentTime)
		return true
	}

	// entry present, check that current delta is within allowed range
	// update if it is
	if currentTime.Sub(val.(time.Time)) >= deltaDuration {
		log.Infof("checkContentDeltaLRU: CACHE HIT - ACCEPT - User %v in cache, and delta time OK. Content type: %v\n", username, contentType)
		userContentDeltaLRU.Add(ContentDelta{user, contentType}, currentTime)
		return true
	}

	log.Infof("checkContentDeltaLRU: CACHE HIT - REJECT - User %v in cache, delta between posts too short. Content type: %v [ val: %v | present: %v ]", user, contentType, val, present)
	return false

}

// checkUserPostDelta sees if a user is posting at a very small delta.
// If we approve a post, we store a timestamp, and on subsequent approvals
// compare it against the stored timestamp.
func checkUserPostDelta(user int64, username string, timeDeltaSeconds int, status *anaconda.Tweet) bool {
	log.Debugf("userPostDeltaLRU: Checking post delta for user %v. Delta is set to %v seconds.\n", user, timeDeltaSeconds)

	currentTime, deltaDuration := calculateTweetTime(status, timeDeltaSeconds)

	val, present := userPostDeltaLRU.Get(user)

	// Not in LRU, likely the first time we have seen the user, or
	// LRU has evicted the entry
	if !present {
		log.Infof("userPostDeltaLRU: CACHE MISS - ACCEPT - User %v not in delta LRU cache.\n", username)
		userPostDeltaLRU.Add(user, currentTime)
		return true
	}

	// check against required delta seconds
	if currentTime.Sub(val.(time.Time)) >= deltaDuration {
		log.Infof("userPostDeltaLRU: CACHE HIT - ACCEPT - User %v in cache, and delta time OK.\n", username)
		userPostDeltaLRU.Add(user, currentTime)
		return true
	}

	log.Infof("userPostDeltaLRU: CACHE HIT - REJECT - User %v in cache, delta between posts too short.", user)
	return false
}

// checkPostRecentLRU stores a recent set of tweet statuses
// if one matches, we can stop processing where this matches
func checkPostRecentLRU(statusText string) bool {

	_, present := postTextLRU.Get(statusText)

	if !present {
		log.Info("checkPostRecentLRU: CACHE MISS - ACCEPT - LRU has not seen this tweet yet.")
		postTextLRU.Add(statusText, 1)
		return true
	}

	log.Info("checkPostRecentLRU: CACHE HIT - REJECT - LRU has seen this post already.")
	return false

}

// checkUserMuted queries to see whether or not a user is
// currently being muted.
func userIsMuted(uid interface{}, mutedIds *memberset.MemberSet) bool {
	// If account is muted, do not process tweets
	if ok := mutedIds.Get(uid); ok {
		log.Info("userIsMuted: - REJECT -  User is muted. Not processing.")
		return true
	}

	log.Info("userIsMuted: - OK - User is not muted.")
	return false
}
