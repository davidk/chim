package main

import (
	"github.com/ChimeraCoder/anaconda"
	"github.com/davidk/memberset"
	"testing"
)

func TestCheckForProhibitedMentions(t *testing.T) {

	// Test notes: prohibitedmentions blocks both ExtendedTweet and regular
	// Entities entries.

	var prohibitedMentionsTest *memberset.MemberSet = memberset.New()

	for _, entries := range []string{"cake", "bank", "ace", "boron"} {
		prohibitedMentionsTest.Add(entries)
	}

	// Test for false-positives in regular Entities
	result := checkForProhibitedMentions(
		anaconda.Tweet{
			CreatedAt:         "Wed Aug 27 13:08:45 +0000 2008",
			Text:              "I have cake!",
			PossiblySensitive: false,
			User: anaconda.User{
				Id:         1337821,
				ScreenName: "bothFollow",
				CreatedAt:  "Wed Aug 27 13:08:45 +0000 2008"},
			Entities: anaconda.Entities{
				User_mentions: []struct {
					Name       string "json:\"name\""
					Indices    []int  "json:\"indices\""
					ScreenName string "json:\"screen_name\""
					ID         int64  "json:\"id\""
					IDStr      string "json:\"id_str\""
				}{
					{Name: "cake", ScreenName: "pie"},
					{Name: "emdawg", ScreenName: "em"},
				},
			},
		}, prohibitedMentionsTest)

	// Expected result: True, pass
	if !result {
		t.Error("pie should be allowed to be mentioned")
	}

	// Test for false negative in ExtendedEntities
	result = checkForProhibitedMentions(
		anaconda.Tweet{
			CreatedAt:         "Wed Aug 27 13:08:45 +0000 2008",
			Text:              "I have cake!",
			PossiblySensitive: false,
			User: anaconda.User{
				Id:         1337821,
				ScreenName: "bothFollow",
				CreatedAt:  "Wed Aug 27 13:08:45 +0000 2008"},
			ExtendedTweet: anaconda.ExtendedTweet{
				ExtendedEntities: anaconda.Entities{
					User_mentions: []struct {
						Name       string "json:\"name\""
						Indices    []int  "json:\"indices\""
						ScreenName string "json:\"screen_name\""
						ID         int64  "json:\"id\""
						IDStr      string "json:\"id_str\""
					}{
						{Name: "pie", ScreenName: "boron"},
						{Name: "hyper", ScreenName: "iron_man"},
					}, //closes struct
				}, //closes Entities
			}, //closes ExtendedTweet
		}, prohibitedMentionsTest)

	// Expected result: False, fail
	if result {
		t.Error("Screen name boron should be blocked based on its entry in the filtered mentions list")
	}

	// Test against false negative, but on mixed case as well
	result = checkForProhibitedMentions(
		anaconda.Tweet{
			CreatedAt:         "Wed Aug 27 13:08:45 +0000 2008",
			Text:              "I have cake!",
			PossiblySensitive: false,
			User: anaconda.User{
				Id:         1337821,
				ScreenName: "bothFollow",
				CreatedAt:  "Wed Aug 27 13:08:45 +0000 2008"},
			ExtendedTweet: anaconda.ExtendedTweet{
				ExtendedEntities: anaconda.Entities{
					User_mentions: []struct {
						Name       string "json:\"name\""
						Indices    []int  "json:\"indices\""
						ScreenName string "json:\"screen_name\""
						ID         int64  "json:\"id\""
						IDStr      string "json:\"id_str\""
					}{
						{Name: "pie", ScreenName: "boron"},
						{Name: "hyper", ScreenName: "iron_man"},
					}, //closes struct
				}, //closes Entities
			}, //closes ExtendedTweet
		}, prohibitedMentionsTest)

	// Expected result: False, fail
	if result {
		t.Error("Screen name boron should be blocked based on its entry in the filtered mentions list (boron != Sunry in filter).")
	}

}

func TestCheckForProhibitedWords(t *testing.T) {
	var prohibitedWordsTest *memberset.MemberSet = memberset.New()

	// note: "dog in the iron" is an invalid entry
	// prohibitedWords is limited in scope only to filter words,
	// and should not attempt lexical analysis (should be reserved for
	// higher-level code)
	for _, entries := range []string{"potassium", "cat", "dog in the iron", "woods", ""} {
		prohibitedWordsTest.Add(entries)
	}

	// Expected result: True, pass
	result := checkForProhibitedWords(anaconda.Tweet{Text: "waffle"}, prohibitedWords)

	if !result {
		t.Error("Check for prohibited words returned false when we wanted true.")
	}

	// Expected result: True, pass
	result = checkForProhibitedWords(anaconda.Tweet{Text: "dog in the iron"}, prohibitedWordsTest)

	if !result {
		t.Error("ProhibitedWords should only filter on individual words, but a full set of text is being blocked")
	}

	// Expected result: False, fail
	result = checkForProhibitedWords(anaconda.Tweet{Text: "cat"}, prohibitedWordsTest)

	if result {
		t.Error("ProhibitedWords failed to block the word cat (listed in the test)")
	}

	// Expected result: True, pass
	result = checkForProhibitedWords(anaconda.Tweet{Text: ""}, prohibitedWordsTest)
	if !result {
		t.Error("ProhibitedWords failed to block the word \"\" (listed in the test)")
	}

}
