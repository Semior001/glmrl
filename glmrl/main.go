package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/Semior001/glmrl/pkg/cmd"
	"github.com/Semior001/glmrl/pkg/git/engine"
	"github.com/Semior001/glmrl/pkg/misc"
	"github.com/Semior001/glmrl/pkg/service"
	"github.com/hashicorp/logutils"
	"github.com/jessevdk/go-flags"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"gopkg.in/yaml.v3"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
)

type options struct {
	Config string `short:"c" long:"config" description:"path to config file" default:"~/.glmrl/config.yaml"`
	Gitlab struct {
		BaseURL string `yaml:"base_url" long:"base-url" env:"BASE_URL" description:"gitlab host"`
		Token   string `yaml:"token" long:"token" env:"TOKEN" description:"gitlab token with read_api scope"`
	} `yaml:"gitlab" group:"gitlab" namespace:"gitlab" env-namespace:"GITLAB"`
	List  cmd.List `yaml:"-" command:"list" description:"list pull requests"`
	Debug bool     `long:"dbg" env:"DEBUG" description:"turn on debug mode"`
	Trace struct {
		Enabled bool   `long:"enabled" env:"ENABLED" description:"enable tracing"`
		Host    string `long:"host" env:"HOST" description:"jaeger agent host"`
		Port    string `long:"port" env:"PORT" description:"jaeger agent port"`
	} `yaml:"-" group:"trace" namespace:"trace" env-namespace:"TRACE"`
}

var version = "unknown"

func getVersion() string {
	v, ok := debug.ReadBuildInfo()
	if !ok || v.Main.Version == "(devel)" || v.Main.Version == "" {
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
		initTracing(opts.Trace.Enabled, getVersion(), opts.Trace.Host, opts.Trace.Port)

		opts = loadConfig(opts.Config, opts)

		copts, err := initCommon(opts)
		if err != nil {
			return fmt.Errorf("init common options: %w", err)
		}

		c.(interface{ Set(cmd.CommonOpts) }).Set(copts)

		if err = c.Execute(args); err != nil {
			return fmt.Errorf("failed to execute command: %w", err)
		}

		return nil
	}

	if _, err := p.Parse(); err != nil {
		var flagsErr *flags.Error
		if errors.As(err, &flagsErr) && flagsErr.Type == flags.ErrHelp {
			os.Exit(0)
		}
		os.Exit(1)
	}
}

func loadConfig(path string, opts options) options {
	if len(path) == 0 {
		return opts
	}

	if path[:2] == "~/" {
		path = filepath.Join(os.Getenv("HOME"), path[2:])
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

	c := cmd.CommonOpts{
		Version: getVersion(),
		PrepareService: func(ctx context.Context) (*service.Service, error) {
			gl, err := engine.NewGitlab(opts.Gitlab.Token, opts.Gitlab.BaseURL, getVersion())
			if err != nil {
				return nil, fmt.Errorf("init gitlab client: %w", err)
			}

			eng := engine.NewInterfaceWithTracing(gl, "Gitlab", misc.AttributesSpanDecorator)

			return service.NewService(ctx, eng)
		},
	}

	return c, nil
}

func setupLog(dbg bool) {
	filter := &logutils.LevelFilter{
		Levels:   []logutils.LogLevel{"DEBUG", "INFO", "WARN", "ERROR"},
		MinLevel: "INFO",
		Writer:   io.Discard,
	}

	logFlags := log.Ltime

	if dbg {
		logFlags = log.Ltime | log.Lmicroseconds | log.Lshortfile
		filter.MinLevel = "DEBUG"

		f, err := os.OpenFile("glmrl.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			log.Fatalf("[ERROR] error opening log file: %v", err)
		}

		// TODO: close file

		filter.Writer = f
	}

	log.SetFlags(logFlags)
	log.SetOutput(filter)
}

func initTracing(enabled bool, version, host, port string) {
	if !enabled {
		return
	}

	opts := []sdktrace.TracerProviderOption{
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.AlwaysSample())),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("glmrl"),
			semconv.ServiceVersionKey.String(version),
		)),
	}

	if enabled {
		je, err := jaeger.New(jaeger.WithAgentEndpoint(
			jaeger.WithAgentHost(host),
			jaeger.WithAgentPort(port),
			jaeger.WithLogger(log.Default()),
		))
		if err != nil {
			log.Fatalf("[ERROR] failed to init jaeger exporter: %v", err)
		}
		// TODO: close exporter

		opts = append(opts, sdktrace.WithBatcher(je))
	}

	tp := sdktrace.NewTracerProvider(opts...)
	otel.SetTracerProvider(tp)
}
