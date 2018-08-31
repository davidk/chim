# Chim

This was a Golang-based Twitter bot designed for re-tweeting gifs and videos out of multiple users' streams / search terms. It was mostly used for quickly collecting/centralizing/archiving community gif/video clippings and contributions onto one account.

As of this bot's public release, it has been in quiet, near-continous operation for over two years; re-tweeting videos and gifs over 9,000 times.

The Twitter API this bot uses is slowly degrading into shutdown, so i'm releasing this for the Internet to archive, and for my own future reference (there are useful parts, still).

The sections below detail this bot's design, build, and operation.

# Features and Non-Features

This bot was originally designed to be low maintenance and to run efficiently/reliably with limited resources. For part of its life, an early version of this bot lived on a desk, quietly sipping CPU time on a Pi Zero.

There were some sacrifices that were made for this:

* No significant state (aside from the config) was kept on disk (to preserve flash program/erase cycles); everything was cached in memory.

* Checks were mostly rules/rate-based.

* Only top-posted ("sent out to my followers") tweets with gifs / videos were checked. Replies were ignored.

* The bot did not inspect the actual text/gif/video's contents.

# Trust Model

With the above limitations, this bot used a mutual follow trust model; this allows the target of any content to transparently choose (by following the source) whose content is retweeted.

This model continues to work well for the community that the bot was a part of, but it may not work for yours.

# Getting Started

1. Create the configuration file above

2. Grab a binary (or build this from source) and place it somewhere on your hosting system. 

```
cp chim /usr/local/chim/chim
chmod +x /usr/local/chim/chim
```

3. Point `chim` to the configuration file (see here for a [basic config](config.json.md))

4. Run `chim -c config.json`

5. The bot should start and enter a listening loop

6. Wait for gifs/video clips to appear (based on watched values)

7. Watch them get re-tweeted by the bot if they pass the filters

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

Documentation for the options and a working example can be found here: [config.json.md](config.json.md).

# Building/Compiling

```
go get -v github.com/davidk/chim
```
or for isolated development purposes (so it doesn't blow up your regular GOPATH):

```
# Clone this into your GOPATH
# EX: 
$ mkdir -p /dev/shm/chim/src/github.com/davidk/
$ cd /dev/shm/chim/src/github.com/davidk/
$ git clone https://github.com/davidk/chim
$ GOPATH=/dev/shm/chim/src make

```

# Deployment

This can be run without the use of supervision scripts/containers if desired.

To persist across reboots there is a hint for systemd-based systems in (`sample_configs/chim.service`).

Instructions are located in the header of the service file.

# Known Bugs / Desired features

These bugs are known and will be addressed:

* A full restart is required for configuration reloads. Some of it can
be done on-demand/hot.

* On flaky connections the bot can sometimes fail to reconnect to Twitter. The reason
  for this isn't well understood, as it rarely happens

### Desired Features

* When adding mutes, the bot should re-read the Twitter API on a signal

* The logging API is inconsistent between log and logrus (still migrating this)

* The Prometheus metrics server needs to be more configurable, and possibly shut off in the configuration.

* The bot sometimes drops the streaming API connection and spins forever. The cause isn't clear.

* A better configuration language would be good

* Generic spam classifier

* Quality detection with ML/AI/DL

# Libraries used to implement this bot

Language: [golang](https://golang.org/)

Twitter Streaming API: [Twitter Streaming API](https://dev.twitter.com/streaming/overview)

Twitter Library (Anaconda): [Anaconda](https://github.com/ChimeraCoder/anaconda)

LRU (a concurrent-access safe version was made): From [groupcache](https://github.com/golang/groupcache)
