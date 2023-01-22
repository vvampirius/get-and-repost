package main

import (
	"os"
	"path"
)

type Core struct {
	ConfigFile *ConfigFile
	Fetchers   []*Fetcher
}

func (core *Core) Reload() {
	if core.Fetchers != nil {
		for _, fetcher := range core.Fetchers {
			fetcher.Cancel()
		}
	}
	core.Fetchers = make([]*Fetcher, 0)
	for name, config := range core.ConfigFile.Config.Get {
		if fetcher, err := NewFetcher(name, config, path.Join(core.ConfigFile.Config.Store, name)); err == nil {
			core.Fetchers = append(core.Fetchers, fetcher)
		}
	}
}

func NewCore(configFile *ConfigFile) (*Core, error) {
	if err := os.MkdirAll(configFile.Config.Store, 0744); err != nil {
		ErrorLog.Println(err.Error())
		return nil, err
	}

	core := Core{
		ConfigFile: configFile,
	}
	core.Reload()
	configFile.OnReload = core.Reload

	return &core, nil
}
