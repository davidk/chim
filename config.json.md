# Configuration

## I just want a working configuration

Head on over to [Twitter Apps](https://apps.twitter.com) with a valid/logged in Twitter account and make a new application.

You will need the following:

* Consumer Key (config: consumer_key)

* Consumer Secret (config: consumer_secret)

* Access Token (config: access_token)

* Access Token Secret (config: access_token_secret)

Here's a somewhat sane config that will watch all tweets from `thecatreviewer` as well as pull in any tweets for kittens, dogs, and cats:

```json
{
  "consumer_key": "",
  "consumer_secret": "",
  "access_token": "",
  "access_token_secret": "",
  "search_terms": "kittens,dogs,cats",
  "watch_users": "thecatreviewer",
  "settings": {
    "must_follow": "",
    "ignore_from": "",
    "post_time_delta_seconds": 5,
    "delta_gated_content_time_seconds": 3600,
    "delta_gated_content": [""],
    "deny_sensitive_content": true,
    "min_account_age_hours": 30,
    "mutual_follow": true,
    "prohibited_mentions": [""],
    "prohibited_words": [""],
    "twitter_filter_level": "none"
  }
}
```

## I really want to know what each of the knobs do

### Twitter authentication elements

These configurations are not optional and are required to successfully connect to Twitter.

Check out [https://apps.twitter.com/](https://apps.twitter.com/) with a valid Twitter
account (and make a new application) for more information.

The settings below can be found in the "keys and access tokens" tab of your Twitter app.

* consumer_key

* consumer_secret

* access_token

* access_token_secret

### Bot configuration

#### search_terms

Example: "search_terms": "iron,aluminium,aluminum,waffle"

A comma separated list of keywords for Twitter to track and filter for the bot.

This is fed to Twitter's "track" parameter: https://developer.twitter.com/en/docs/tweets/filter-realtime/api-reference/post-statuses-filter.html

#### watch_users

Example: "watch_users": "thecatreviewer,dog_rates,thedogiest,Bodegacats_"

A comma separted list of users to always retrieve results from

https://developer.twitter.com/en/docs/tweets/filter-realtime/api-reference/post-statuses-filter.html

#### logrus_level

Example: "logrus_level"  

Directly affects logrus' logging output. These map to logrus' `log.DebugLevel`, etc. Possible options are:

* debug

* info

* warning

* error

* fatal

* panic

The default level is "debug".

#### test_mode

Example: "test_mode": false

Setting test_mode causes all tweets to be processed, but does not actually retweet them

#### settings

The nested settings{} dictionary controls the bot's filtering behavior. These are tuned above for low volume
retweet potentials.

Example:

```
{
  "settings": {
    "mutual_follow": true,
    "ignore_from": someone
  }
}
```

#### must_follow

Example: must_follow: some_account

A string value for an account. The tweet sender must follow this account in order to pass checks.

Note: This is a rate-limited call, so entries are cached in an LRU.

https://developer.twitter.com/en/docs/accounts-and-users/follow-search-get-users/api-reference/get-users-show

#### mutual_follow

Example: mutual_follow: true

This parameter is true/false. It checks for a two-way following relationship, so
the target and source of the tweet must be following each other in order
for a retweet to occur.

#### ignore_from

Example: ignore_from: "bot"

A single user to ignore tweets from. This is/was an early feature that has been nearly
replaced by muting instead. Documented here for completeness (and may be fleshed out in the future).

#### post_time_delta_seconds

Example: post_time_delta_seconds: 120

The time in seconds that a user must wait before we accept a new status to retweet.

#### delta_gated_content

Example: "delta_gated_content": ["video", "gif"]

A list of content types that have a separate time delta applied to them (the tweet will also pass through post_time_delta_seconds).

#### delta_gated_content_time_seconds

Example: delta_gated_content_time_seconds: 3600

Specifies the time in seconds between postings where we will accept a new status to retweet.

If the time hasn't elapsed, the tweet is dropped and not re-tweeted.

#### deny_sensitive_content

Example: deny_sensitive_content: true

This is a true/false parameter and will filter content that is marked 'sensitive' on Twitter's end.

#### min_account_age_hours

Example: min_account_age_hours: 5

The age of an account before we can be receptive to any retweets.

#### prohibited_mentions

Example: prohibited_mentions: ["npr", "twitter", "jack"]

If a tweet mentions any users listed in prohibited_mentions, it is rejected.
Mentions are contained in Twitter's stream API response, and are not parsed locally.

#### prohibited_words

Example: prohibited_words: ["cake", "pie", "waffle"]

If a tweet mentions any words listed, it is rejected. A very basic check.

#### twitter_filter_level

Example: twitter_filter_level: "none"

This is a twitter ML setting that potentially makes content more consumable for the public. Currently most tweets are "low".

Possible values, per the documentation: none, low, medium

"high" is not yet implemented

https://developer.twitter.com/en/docs/tweets/filter-realtime/guides/basic-stream-parameters
