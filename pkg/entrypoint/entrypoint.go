package entrypoint

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"syscall"

	"github.com/wrouesnel/proxyreverse/assets"
	"github.com/wrouesnel/proxyreverse/pkg/server"
	"github.com/wrouesnel/proxyreverse/pkg/server/config"

	"github.com/alecthomas/kong"
	"github.com/samber/lo"
	"github.com/wrouesnel/proxyreverse/version"
	"go.uber.org/zap"
)

type Options struct {
	Logging struct {
		Level  string `help:"logging level" default:"warning"`
		Format string `help:"logging format (${enum})" enum:"console,json" default:"console"`
	} `embed:"" prefix:"logging."`

	Config string `help:"File to load config from" default:"proxyreverse.yml"`

	Assets assets.Config `embed:"" prefix:"assets." help:"configure embedded asset handling"`

	Version bool `help:"Print the version and exit"`

	ReverseProxy server.ServerCommand `cmd:"" help:"Start proxyreverse server"`
	DumpConfig   struct{}             `cmd:"" help:"Dump active configuration"`
}

type LaunchArgs struct {
	StdIn  io.Reader
	StdOut io.Writer
	StdErr io.Writer
	Env    map[string]string
	Args   []string
}

// Entrypoint implements the actual functionality of the program so it can be called inline from testing.
// env is normally passed the environment variable array.
//
//nolint:funlen,gocognit,gocyclo,cyclop,maintidx
func Entrypoint(args LaunchArgs) int {
	var err error
	options := Options{}

	deferredLogs := []string{}

	// Command line parsing can now happen
	parser := lo.Must(kong.New(&options, kong.Description(version.Description),
		kong.DefaultEnvars(version.EnvPrefix)))
	ctx, err := parser.Parse(args.Args)
	if err != nil {
		_, _ = fmt.Fprintf(args.StdErr, "Argument error: %s", err.Error())
		return 1
	}

	// Initialize logging as soon as possible
	logConfig := zap.NewProductionConfig()
	if err := logConfig.Level.UnmarshalText([]byte(options.Logging.Level)); err != nil {
		deferredLogs = append(deferredLogs, err.Error())
	}
	logConfig.Encoding = options.Logging.Format

	logger, err := logConfig.Build()
	if err != nil {
		// Error unhandled since this is a very early failure
		for _, line := range deferredLogs {
			_, _ = io.WriteString(args.StdErr, line)
		}
		_, _ = io.WriteString(args.StdErr, "Failure while building logger")
		return 1
	}

	// Install as the global logger
	zap.ReplaceGlobals(logger)

	logger.Info("Launched with command line", zap.Strings("cmdline", args.Args))

	if options.Version {
		lo.Must(fmt.Fprintf(args.StdOut, "%s", version.Version))
		return 0
	}

	logger.Info("Version Info", zap.String("version", version.Version),
		zap.String("name", version.Name),
		zap.String("description", version.Description),
		zap.String("env_prefix", version.EnvPrefix))

	appCtx, cancelFn := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM)

	go func() {
		for sig := range sigCh {
			logger.Info("Caught signal", zap.String("signal", sig.String()))
			cancelFn()
			return
		}
	}()

	logger = logger.With(zap.String("command", ctx.Command()), zap.String("config_file", options.Config))

	logger.Info("Parsing configuration")
	configBytes, err := ioutil.ReadFile(options.Config)
	if err != nil {
		logger.Error("Error loading config", zap.Error(err))
		return 1
	}

	cfg, err := config.Load(configBytes)
	if err != nil {
		logger.Error("Error loading config", zap.Error(err))
		return 1
	}

	sanitizedCfg, err := config.LoadAndSanitizeConfig(configBytes)
	if err != nil {
		logger.Error("Error loading config", zap.Error(err))
		return 1
	}

	logger.Info("Starting command")
	switch ctx.Command() {
	case "reverse-proxy":
		err = server.Server(appCtx, options.Assets, options.ReverseProxy, cfg)
	case "dump-config":
		args.StdOut.Write([]byte(sanitizedCfg))
	default:
		logger.Error("Command not implemented")
	}

	logger.Debug("Finished command")
	if err != nil {
		logger.Error("Command exited with error", zap.Error(err))
		return 1
	}

	return 0
}
