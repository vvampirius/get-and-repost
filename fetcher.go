package main

import (
	"context"
	"errors"
	"github.com/robfig/cron/v3"
	"io"
	"net/http"
	"os"
	"time"
)

type Fetcher struct {
	Name         string
	Config       ConfigGet
	Path         string
	cancelFetch  context.CancelFunc
	cancelRepost map[string]context.CancelFunc
}

func (fetcher *Fetcher) Start() error {
	schedule, err := cron.ParseStandard(fetcher.Config.Cron)
	if err != nil {
		ErrorLog.Println(fetcher.Config.Cron, err.Error())
		return err
	}
	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	fetcher.cancelFetch = cancelFunc
	go func() {
		for {
			fetcher.Fetch(cancelCtx)
			waitCtx, _ := context.WithDeadline(context.Background(), schedule.Next(time.Now()))
			select {
			case <-waitCtx.Done():
				continue
			case <-cancelCtx.Done():
				DebugLog.Println(`выходим`)
				return
			}
		}
	}()
	return nil
}

func (fetcher *Fetcher) Cancel() {
	if fetcher.cancelFetch != nil {
		fetcher.cancelFetch()
	}
}

func (fetcher *Fetcher) checkDate(responseDate string) error {
	if fetcher.Config.FreshnessMethod != `date` {
		return nil
	}
	fileInfo, err := os.Stat(fetcher.Path)
	if err != nil {
		return nil
	}
	date, err := time.Parse(time.RFC1123, responseDate)
	if err != nil {
		return nil
	}
	if !date.After(fileInfo.ModTime()) {
		return errors.New(`Is not newer`)
	}
	return nil
}

func (fetcher *Fetcher) fetchToTemporaryFile(cancelCtx context.Context) (string, error) {
	f, err := os.CreateTemp(``, `get-and-repost_`)
	if err != nil {
		ErrorLog.Println(err.Error())
		return "", err
	}
	defer f.Close()

	ctx, _ := context.WithTimeout(cancelCtx, 3*time.Second)
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, fetcher.Config.Url, nil)
	if err != nil {
		ErrorLog.Println(fetcher.Name, fetcher.Config.Url, err.Error())
		return f.Name(), err
	}

	for k, v := range fetcher.Config.Headers {
		r.Header.Add(k, v)
	}

	client := http.Client{}
	response, err := client.Do(r)
	if err != nil {
		ErrorLog.Printf("Error making request for target '%s' with url '%s': %s\n", fetcher.Name,
			fetcher.Config.Url, err.Error())
		return f.Name(), err
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		ErrorLog.Println(fetcher.Name, fetcher.Config.Url, response.Status)
		return f.Name(), errors.New(`Bad response status code`)
	}

	if err = fetcher.checkDate(response.Header.Get(`Date`)); err != nil {
		DebugLog.Println(fetcher.Name, err.Error())
		return f.Name(), err
	}

	if _, err = io.Copy(f, response.Body); err != nil {
		ErrorLog.Println(fetcher.Name, err.Error())
		return f.Name(), err
	}

	return f.Name(), nil
}

func (fetcher *Fetcher) checkSize(tempFilePath string) error {
	if fetcher.Config.FreshnessMethod != `size` {
		return nil
	}
	lastFinfo, err := os.Stat(fetcher.Path)
	if err != nil {
		return nil
	}
	currentFinfo, _ := os.Stat(tempFilePath)
	if lastFinfo.Size() == currentFinfo.Size() {
		DebugLog.Println(fetcher.Name, `is equal`)
		return errors.New(`is equal`)
	}
	return nil
}

func (fetcher *Fetcher) Fetch(cancelCtx context.Context) {
	DebugLog.Printf("Fetching: %s => %s\n", fetcher.Name, fetcher.Config.Url)
	tempFilePath, err := fetcher.fetchToTemporaryFile(cancelCtx)
	if tempFilePath != `` {
		defer os.Remove(tempFilePath)
	}
	if err != nil {
		return
	}
	if err = fetcher.checkSize(tempFilePath); err != nil {
		return
	}
	for _, cancel := range fetcher.cancelRepost {
		cancel()
	}
	os.Remove(fetcher.Path)
	if err = os.Rename(tempFilePath, fetcher.Path); err != nil {
		ErrorLog.Println(fetcher.Name, err.Error())
		return
	}
	for name, config := range fetcher.Config.Repost {
		fetcher.Repost(name, config)
	}
}

func (fetcher *Fetcher) Repost(name string, config ConfigRepost) {
	DebugLog.Printf("Repost %s to %s (%s)\n", fetcher.Name, name, config.Url)
	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	fetcher.cancelRepost[name] = cancelFunc
	go func() {
		for {
			f, err := os.Open(fetcher.Path)
			if err != nil {
				ErrorLog.Println(fetcher.Name, err.Error())
				return
			}

			ctx, _ := context.WithTimeout(cancelCtx, 3*time.Second)
			r, err := http.NewRequestWithContext(ctx, http.MethodPost, config.Url, f)
			if err != nil {
				ErrorLog.Println(fetcher.Name, name, config.Url, err.Error())
				f.Close()
				return
			}

			client := http.Client{}
			response, err := client.Do(r)
			if err != nil {
				ErrorLog.Println(fetcher.Name, name, config.Url, err.Error())
				f.Close()
				return
			}

			f.Close()

			if response.StatusCode != 200 {
				ErrorLog.Println(fetcher.Name, name, config.Url, err.Error())
				response.Body.Close()
				return
			}

			response.Body.Close()

			select {
			case <-time.After(time.Minute):
				continue
			case <-ctx.Done():
				return
			}
		}
	}()
}

func NewFetcher(name string, config ConfigGet, path string) (*Fetcher, error) {
	fetcher := Fetcher{
		Name:         name,
		Config:       config,
		Path:         path,
		cancelRepost: make(map[string]context.CancelFunc),
	}
	if err := fetcher.Start(); err != nil {
		return nil, err
	}
	return &fetcher, nil
}
