package main

import (
	"github.com/ChimeraCoder/anaconda"
	"github.com/davidk/memberset"
	log "github.com/sirupsen/logrus"
	"net/url"
)

// FriendshipInfo is replaced in testing with fake versions that
// do not call out to the Twitter API. A FriendshipInfo is passed to
// checkUserFollowing and calls api.GetFriendshipsShow by proxy

// GetMutedList wraps Anaconda's GetMutedUsersIds for testing
type GetMutedList interface {
	GetMutedUsersList(v url.Values) (c anaconda.UserCursor, err error)
}

// MutedInfo stubs out Anaconda's information for testing / production
type MutedInfo struct{}

// GetMutedUsersList passes to Anaconda's GetMutedUsersList()
func (fs MutedInfo) GetMutedUsersList(v url.Values) (c anaconda.UserCursor, err error) {
	return api.GetMutedUsersList(v)
}

// populateMutedList grabs the muted user list from the API and stores it.
func populateMutedList(m GetMutedList, v url.Values, mutedIds *memberset.MemberSet) {

	log.Println("populateMutedList: Requesting list of muted user IDs from API.")
	cursor, err := m.GetMutedUsersList(v)
	check(ErrorsAreFatal{}, "Unable to get list of muted user IDs from API", err)
	for _, user := range cursor.Users {
		log.Println("Got user:", user.Id)
		mutedIds.Add(user.Id)
		log.Printf("populateMutedList: Muting tweets from: %v [id: %v ]\n", user.ScreenName, user.Id)
	}

	if cursor.Next_cursor_str != "0" {
		values := url.Values{}
		log.Printf("populateMutedList: Retrieving next set of users ( recursion active, next cursor: %v )\n", cursor.Next_cursor_str)
		values.Set("cursor", cursor.Next_cursor_str)
		populateMutedList(m, values, mutedIds)
	}

	log.Println("populateMutedList: Done.")
}
