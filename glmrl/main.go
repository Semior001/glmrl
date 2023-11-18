package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/Semior001/glmrl/pkg/cmd"
	"github.com/Semior001/glmrl/pkg/git/engine"
	"github.com/Semior001/glmrl/pkg/service"
	"github.com/hashicorp/logutils"
	"github.com/jessevdk/go-flags"
	"gopkg.in/yaml.v3"
	"log"
	"os"
	"runtime/debug"
)

type options struct {
	Config string `short:"c" long:"config" description:"path to config file" default:"~/.glmrl/config.yaml"`
	Gitlab struct {
		BaseURL string `yaml:"base_url" long:"base-url" env:"BASE_URL" description:"gitlab host"`
		Token   string `yaml:"token" long:"token" env:"TOKEN" description:"gitlab token"`
	} `yaml:"gitlab" group:"gitlab" namespace:"gitlab" env-namespace:"GITLAB"`
	List  cmd.List `yaml:"-" command:"list" description:"list pull requests"`
	Debug bool     `long:"dbg" env:"DEBUG" description:"turn on debug mode"`
}

var version = "unknown"

func getVersion() string {
	v, ok := debug.ReadBuildInfo()
	if !ok || v.Main.Version == "(devel)" {
		return version
	}
	return v.Main.Version
}

func main() {
	fmt.Printf("glmrl version: %s\n", getVersion())

	opts := options{}

	p := flags.NewParser(&opts, flags.Default)
	p.CommandHandler = func(c flags.Commander, args []string) error {
		setupLog(opts.Debug)

		opts = loadConfig(opts.Config, opts)

		copts, err := initCommon(opts)
		if err != nil {
			return fmt.Errorf("init common options: %w", err)
		}

		c.(interface{ Set(cmd.CommonOpts) }).Set(copts)

		if err = c.Execute(args); err != nil {
			log.Printf("[ERROR] failed to execute command: %+v", err)
		}

		return nil
	}

	if _, err := p.Parse(); err != nil {
		if flagsErr, ok := err.(*flags.Error); ok && flagsErr.Type == flags.ErrHelp {
			os.Exit(0)
		}
		os.Exit(1)
	}
}

func loadConfig(path string, opts options) options {
	// resolve "~" in path
	if path[:2] == "~/" {
		path = os.Getenv("HOME") + path[1:]
	}

	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Printf("[WARN] didn't find config at %s", path)
			return opts
		}
		log.Printf("[WARN] failed to open config at %s: %v", path, err)
		return opts
	}
	defer file.Close()

	var cfg options
	if err = yaml.NewDecoder(file).Decode(&cfg); err != nil {
		log.Printf("[WARN] failed to decode config at %s: %v", path, err)
		return opts
	}

	opts.Gitlab = cfg.Gitlab
	return opts
}

func initCommon(opts options) (cmd.CommonOpts, error) {
	if opts.Gitlab.Token == "" && opts.Gitlab.BaseURL == "" {
		return cmd.CommonOpts{}, errors.New("gitlab creds not provided")
	}

	gl, err := engine.NewGitlab(opts.Gitlab.Token, opts.Gitlab.BaseURL)
	if err != nil {
		return cmd.CommonOpts{}, fmt.Errorf("init gitlab client: %w", err)
	}

	svc, err := service.NewService(context.Background(), gl)
	if err != nil {
		return cmd.CommonOpts{}, fmt.Errorf("init service: %w", err)
	}

	return cmd.CommonOpts{Version: getVersion(), Service: svc}, nil
}

func setupLog(dbg bool) {
	f, err := os.OpenFile("glmrl.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("[ERROR] error opening file: %v", err)
	}

	filter := &logutils.LevelFilter{
		Levels:   []logutils.LogLevel{"DEBUG", "INFO", "WARN", "ERROR"},
		MinLevel: "INFO",
		Writer:   f,
	}

	logFlags := log.Ltime

	if dbg {
		logFlags = log.Ltime | log.Lmicroseconds | log.Lshortfile
		filter.MinLevel = "DEBUG"
	}

	log.SetFlags(logFlags)
	log.SetOutput(filter)
}
