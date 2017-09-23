package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mitchellh/go-homedir"
	"github.com/urfave/cli"
)

var errCommandHelp = fmt.Errorf("command help shown")

func main() {
	app := cli.NewApp()
	app.Commands = []cli.Command{
		commandPull,
		commandPush,
		commandPost,
	}
	app.Version = fmt.Sprintf("%s (%s)", version, revision)
	err := app.Run(os.Args)
	if err != nil {
		if err != errCommandHelp {
			logf("error", "%s", err)
		}
		os.Exit(1)
	}
}

func loadSingleConfigFile(fname string) (*config, error) {
	if _, err := os.Stat(fname); err != nil {
		return nil, nil
	}
	f, err := os.Open(fname)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return loadConfig(f)
}

func loadConfigFile() (*config, error) {
	var conf *config

	pwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	conf, err = loadSingleConfigFile(filepath.Join(pwd, "blogsync.yaml"))
	if err != nil {
		return nil, err
	}

	home, err := homedir.Dir()
	if err != nil && conf == nil {
		return nil, err
	}
	if err == nil {
		homeConf, err := loadSingleConfigFile(filepath.Join(home, ".config", "blogsync", "config.yaml"))
		if err != nil {
			return nil, err
		}
		conf = mergeConfig(conf, homeConf)
	}
	if conf == nil {
		return nil, fmt.Errorf("no config files found")
	}
	return conf, nil
}

var commandPull = cli.Command{
	Name:  "pull",
	Usage: "Pull entries from remote",
	Action: func(c *cli.Context) error {
		blog := c.Args().First()
		if blog == "" {
			cli.ShowCommandHelp(c, "pull")
			return errCommandHelp
		}

		conf, err := loadConfigFile()
		if err != nil {
			return err
		}
		blogConfig := conf.Get(blog)
		if blogConfig == nil {
			return fmt.Errorf("blog not found: %s", blog)
		}

		b := newBroker(blogConfig)
		remoteEntries, err := b.FetchRemoteEntries()
		if err != nil {
			return err
		}

		for _, re := range remoteEntries {
			path := b.LocalPath(re)
			_, err := b.StoreFresh(re, path)
			if err != nil {
				return err
			}
		}
		return nil
	},
}

var commandPush = cli.Command{
	Name:  "push",
	Usage: "Push local entries to remote",
	Action: func(c *cli.Context) error {
		path := c.Args().First()
		if path == "" {
			cli.ShowCommandHelp(c, "push")
			return errCommandHelp
		}

		path, err := filepath.Abs(path)
		if err != nil {
			return err
		}

		var blogConfig *blogConfig

		conf, err := loadConfigFile()
		if err != nil {
			return err
		}
		for remoteRoot := range conf.Blogs {
			bc := conf.Get(remoteRoot)
			localRoot, err := filepath.Abs(filepath.Join(bc.LocalRoot, remoteRoot))
			if err != nil {
				return err
			}

			if strings.HasPrefix(path, localRoot) {
				blogConfig = bc
				break
			}
		}

		if blogConfig == nil {
			return fmt.Errorf("cannot find blog for %s", path)
		}

		b := newBroker(blogConfig)

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		entry, err := entryFromReader(f)
		if err != nil {
			return err
		}

		b.UploadFresh(entry)
		return nil
	},
}

var commandPost = cli.Command{
	Name:  "post",
	Usage: "Post a new entry to remote",
	Flags: []cli.Flag{
		cli.BoolFlag{Name: "draft"},
		cli.StringFlag{Name: "title"},
		cli.StringFlag{Name: "custom-path"},
	},
	Action: func(c *cli.Context) error {
		blog := c.Args().First()
		if blog == "" {
			cli.ShowCommandHelp(c, "post")
			return errCommandHelp
		}

		conf, err := loadConfigFile()
		if err != nil {
			return err
		}
		blogConfig := conf.Get(blog)
		if blogConfig == nil {
			return fmt.Errorf("blog not found: %s", blog)
		}

		entry, err := entryFromReader(os.Stdin)
		if err != nil {
			return err
		}

		if c.Bool("draft") {
			entry.IsDraft = true
		}

		if path := c.String("custom-path"); path != "" {
			entry.CustomPath = path
		}

		if title := c.String("title"); title != "" {
			entry.Title = title
		}

		b := newBroker(blogConfig)
		err = b.PostEntry(entry)
		if err != nil {
			return err
		}
		return nil
	},
}
