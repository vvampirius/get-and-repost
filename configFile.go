package main

import (
	"gopkg.in/yaml.v2"
	"os"
	"sync"
	"time"
)

type ConfigFile struct {
	FilePath     string
	FileModified time.Time
	Config       *Config
	Mutex        sync.Mutex
	OnReload     func()
}

func (configFile *ConfigFile) Save() error {
	configFile.Mutex.Lock()
	defer configFile.Mutex.Unlock()

	f, err := os.OpenFile(configFile.FilePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644) //TODO: get perm from struct
	if err != nil {
		ErrorLog.Println(configFile.FilePath, err.Error())
		return err
	}

	encoder := yaml.NewEncoder(f)
	if err := encoder.Encode(configFile.Config); err != nil {
		ErrorLog.Println(configFile.Config, err.Error())
		f.Close()
		return err
	}

	f.Close()
	configFile.FileModified = time.Now()
	return nil
}

func (configFile *ConfigFile) Reload() error {
	configFile.Mutex.Lock()
	defer configFile.Mutex.Unlock()

	f, err := os.Open(configFile.FilePath)
	if err != nil {
		ErrorLog.Println(configFile.FilePath, err.Error())
		return err
	}
	defer f.Close()

	config := Config{}

	decoder := yaml.NewDecoder(f)
	if err := decoder.Decode(&config); err != nil {
		ErrorLog.Println(configFile.FilePath, err.Error())
		return err
	}

	configFile.Config = &config
	configFile.FileModified = configFile.GetFileMTime()
	if configFile.OnReload != nil {
		configFile.OnReload()
	}
	return nil
}

func (configFile *ConfigFile) GetFileMTime() time.Time {
	fileInfo, err := os.Stat(configFile.FilePath)
	if err != nil {
		return time.Time{}
	}
	return fileInfo.ModTime()
}

func (configFile *ConfigFile) ReloadRoutine() {
	for {
		time.Sleep(30 * time.Second)
		if configFile.GetFileMTime().After(configFile.FileModified) {
			DebugLog.Printf("%s updated. Reloading config file...\n", configFile.FilePath)
			configFile.Reload()
		}
	}
}

func NewConfigFile(filePath string) (*ConfigFile, error) {
	configFile := ConfigFile{
		FilePath: filePath,
	}
	if err := configFile.Reload(); err != nil {
		return nil, err
	}
	go configFile.ReloadRoutine()
	return &configFile, nil
}
