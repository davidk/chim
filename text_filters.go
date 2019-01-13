// Filter out any text that might be problematic pre configuration
// on start, this is loaded into a memberset which we can check the
// status update against.
package main

import (
	"github.com/ChimeraCoder/anaconda"
	"github.com/davidk/memberset"
	log "github.com/sirupsen/logrus"
	"strings"
)

// filterMentions ingests all the mentions parsed by Twitter and
// checks them against a set for membership. Tweets fail this test
// if they are in the set.
func checkForProhibitedMentions(status anaconda.Tweet, filteredMentions *memberset.MemberSet) bool {

	for _, mention := range status.ExtendedTweet.ExtendedEntities.User_mentions {
		log.Println(mention.Screen_name)
		log.Printf("%+v\n", mention)
		if filteredMentions.Get(strings.ToLower(mention.Screen_name)) {
			log.Println("checkForProhibitedMentionsExtended: REJECT - Filtering on screen_name in mention:", mention.Screen_name)
			return false
		}
	}

	for _, mention := range status.Entities.User_mentions {
		log.Println(mention.Screen_name)
		log.Printf("%+v\n", mention)
		if filteredMentions.Get(strings.ToLower(mention.Screen_name)) {
			log.Println("checkForProhibitedMentions: REJECT - Filtering on screen_name in mention:", mention.Screen_name)
			return false
		}
	}

	log.Println("checkForProhibitedMentions: OK")
	return true

}

// filterText loosely checks the text's words for anything that might
// not be entirely permissible.
// True  -- is passing for this test
// False -- means that a prohibited word was found
func checkForProhibitedWords(status anaconda.Tweet, filteredWords *memberset.MemberSet) bool {

	for _, word := range strings.Split(status.Text, " ") {
		if word != "" && filteredWords.Get(word) {
			log.Printf("checkForProhibitedWords: REJECT - Found prohibited word '%v' in text: %v", word, status.ExtendedTweet.FullText)
			return false
		}
	}

	log.Println("checkForProhibitedWords: OK")
	return true

}
