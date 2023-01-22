package main

type ConfigRepost struct {
	Url string
}

type ConfigGet struct {
	Url             string
	Cron            string
	Headers         map[string]string
	Repost          map[string]ConfigRepost
	FreshnessMethod string `yaml:"freshness_method"`
}

type Config struct {
	Listen string
	Store  string
	Get    map[string]ConfigGet
}
