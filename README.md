# Chim

[![Go Report Card](https://goreportcard.com/badge/github.com/davidk/chim)](https://goreportcard.com/report/github.com/davidk/chim) [![Docker hub pulls](https://img.shields.io/docker/pulls/keyglitch/chim.svg?style=plastic)](https://hub.docker.com/r/keyglitch/chim)

This was a Golang-based Twitter bot designed for re-tweeting gifs and videos out of twitter's streaming API, given a set of search terms and hashtags. It was mostly used for quickly collecting/centralizing/archiving community gif/video clippings and contributions onto one account.

As of this bot's public release, it has been in quiet, near-continous operation for over two years; re-tweeting videos and gifs over 10,000 times.

The Twitter API this bot uses is slowly degrading into shutdown, so i'm releasing this for the Internet to archive, and for my own future reference (there are useful parts, still).

The sections below detail this bot's design, build, and operation.

# Features and Non-Features

This bot was originally designed to be low maintenance and to run efficiently/reliably with limited resources. For part of its life, an early version of this bot lived on a desk, quietly sipping CPU time on a Pi Zero.

There were some sacrifices that were made for this:

* No significant state (aside from the config) was kept on disk (to preserve flash program/erase cycles)

* Checks were mostly rules/rate-based.

* Only top-posted, relevant ("sent out to my followers") tweets with gifs / videos were checked. Replies and threads were ignored.

* The bot did not deeply inspect the actual text/gif/video's contents.

* Expensive/rate-limited calls were aggressively cached (to prevent rate-limiting/stalls during large events).

# Trust Model

With the above limitations, this bot used a mutual follow trust model; this allowed the target of any content to transparently choose (by following the source) whose content was retweeted.

This model worked well for the community that the bot was a part of, but it may not work for yours.

# Building/Compiling

```
go get -v github.com/davidk/chim
```
or for isolated development purposes (so it doesn't blow up your regular GOPATH):

```
# Clone this into a temporary "GOPATH"
# EX: 
$ mkdir -p /dev/shm/chim/src/github.com/davidk/
$ cd /dev/shm/chim/src/github.com/davidk/
$ git clone https://github.com/davidk/chim
$ GOPATH=/dev/shm/chim/src make

```

# Getting Started

1. Build/grab a binary and place it somewhere on your hosting system. 

```
cp chim /usr/local/chim/chim
chmod +x /usr/local/chim/chim
```

2. Point `chim` to the configuration file (see below, or [here, for a basic config](config.json.md))

3. Run `chim -c config.json`

4. The bot should start and enter a listening loop

5. Wait for gifs/video clips to appear (based on watched values) and get checked by the filters

6. Watch things get re-tweeted by the bot

# Configuration File

The bot requires a configuration file (named `config.json`) with the following structure in JSON:

```
{
  "consumer_key": "",
  "consumer_secret": "",
  "access_token": "",
  "access_token_secret": "",
  "search_terms": "#hashtags,kittens,cats,#hashes,#hashing",
  "watch_users": "user1,user2,user3",
  "logrus_level": "info",
  "test_mode": true,
  "settings": {
    "must_follow": "aCertainUser",
    "ignore_from": "chimbot",
    "post_time_delta": 5,
    "delta_gated_content_time_seconds": "",
    "delta_gated_content": "",
    "deny_sensitive_content": true
    "min_account_age_hours": "",
    "mutual_follow": "",
    "prohibited_mentions": "",
    "prohibited_words": "",
    "twitter_filter_level": "",
  }
}
```

Note: `"test_mode": true` will run all filtering, but refuse to actually retweet any matching content.

Documentation for the options and a working example can be found here: [config.json.md](config.json.md).

# Deployment

This can be run without the use of supervision scripts/containers if desired.

To persist across reboots there is a hint for systemd-based systems in (`sample_configs/chim.service`).

Instructions are located in the header of the service file.

# Known Bugs / Desired features

These bugs were known:

* A full restart is required for configuration reloads. Some of it can
be done on-demand/hot.

* On flaky connections the bot can sometimes fail to reconnect to Twitter. The reason
  for this isn't well understood, as it rarely happens

### Desired Features

As this bot was being retired:

* When adding mutes, the bot should re-read the Twitter API on a signal

* The logging API is inconsistent between log and logrus

* The Prometheus metrics server needs to be more configurable, and possibly shut off in the configuration.

* The bot sometimes drops the streaming API connection and spins forever. The cause isn't clear.

* A better configuration language would be good

* Generic spam classifier

* Quality detection with ML/AI/DL

# Will this bot ever come out of retirement?

Maybe! Porting to the new API implementation seems substantial right now.

# Main libraries and references used to implement this bot

Language: [golang](https://golang.org/)

Twitter Streaming API: [Twitter Streaming API](https://dev.twitter.com/streaming/overview)

Twitter Library (Anaconda): [Anaconda](https://github.com/ChimeraCoder/anaconda)

LRU (a concurrent-access safe version was made, derived from) [groupcache](https://github.com/golang/groupcache)

