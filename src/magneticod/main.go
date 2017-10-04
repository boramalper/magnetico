package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"regexp"
	"time"

	"github.com/jessevdk/go-flags"
	"github.com/pkg/profile"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"persistence"

	"magneticod/bittorrent"
	"magneticod/dht"
	"github.com/anacrolix/torrent/metainfo"
)

type cmdFlags struct {
	DatabaseURL string `long:"database" description:"URL of the database."`

	TrawlerMlAddrs    []string `long:"trawler-ml-addr" description:"Address(es) to be used by trawling DHT (Mainline) nodes." default:"0.0.0.0:0"`
	TrawlerMlInterval uint     `long:"trawler-ml-interval" description:"Trawling interval in integer deciseconds (one tenth of a second)."`

	// TODO: is this even supported by anacrolix/torrent?
	FetcherAddr    string `long:"fetcher-addr" description:"Address(es) to be used by ephemeral peers fetching torrent metadata." default:"0.0.0.0:0"`
	FetcherTimeout uint   `long:"fetcher-timeout" description:"Number of integer seconds before a fetcher timeouts."`

	StatistMlAddrs   []string `long:"statist-ml-addr" description:"Address(es) to be used by ephemeral nodes fetching latest statistics about individual torrents." default:"0.0.0.0:0"`
	StatistMlTimeout uint     `long:"statist-ml-timeout" description:"Number of integer seconds before a statist timeouts."`

	// TODO: is this even supported by anacrolix/torrent?
	LeechClAddr   string   `long:"leech-cl-addr" description:"Address to be used by the peer fetching README files." default:"0.0.0.0:0"`
	LeechMlAddr   string   `long:"leech-ml-addr"  descrition:"Address to be used by the mainline DHT node for fetching README files." default:"0.0.0.0:0"`
	LeechTimeout  uint     `long:"leech-timeout" description:"Number of integer seconds to pass before a leech timeouts." default:"300"`
	ReadmeMaxSize uint     `long:"readme-max-size" description:"Maximum size -which must be greater than zero- of a description file in bytes." default:"20480"`
	ReadmeRegex   string   `long:"readme-regex" description:"Regular expression(s) which will be tested against the name of the README files, in the supplied order."`

	Verbose []bool `short:"v" long:"verbose" description:"Increases verbosity."`

	Profile string `long:"profile" description:"Enable profiling." default:""`

	// ==== OLD Flags ====

	// DatabaseFile is akin to Database flag, except that it was used when SQLite was the only
	// persistence backend ever conceived, so it's the path* to the database file, which was -by
	// default- located in wherever appdata module on Python said:
	//     On GNU/Linux    : `/home/<USER>/.local/share/magneticod/database.sqlite3`
	//     On Windows      : TODO?
	//     On MacOS (OS X) : TODO?
	//     On BSDs?        : TODO?
	//     On anywhere else: TODO?
	// TODO: Is the path* absolute or can be relative as well?
}

const (
	PROFILE_BLOCK = 1
	PROFILE_CPU
	PROFILE_MEM
	PROFILE_MUTEX
	PROFILE_A
)

type opFlags struct {
	DatabaseURL string

	TrawlerMlAddrs    []string
	TrawlerMlInterval time.Duration

	FetcherAddr    string
	FetcherTimeout time.Duration

	StatistMlAddrs   []string
	StatistMlTimeout time.Duration

	LeechClAddr   string
	LeechMlAddr   string
	LeechTimeout  time.Duration
	ReadmeMaxSize uint
	ReadmeRegex   *regexp.Regexp

	Verbosity int

	Profile string
}

func main() {
	loggerLevel := zap.NewAtomicLevel()
	// Logging levels: ("debug", "info", "warn", "error", "dpanic", "panic", and "fatal").
	logger := zap.New(zapcore.NewCore(
		zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig()),
		zapcore.Lock(os.Stderr),
		loggerLevel,
	))
	defer logger.Sync()
	zap.ReplaceGlobals(logger)

	defer profile.Start(profile.CPUProfile, profile.ProfilePath(".")).Stop()

	zap.L().Info("magneticod v0.7.0 has been started.")
	zap.L().Info("Copyright (C) 2017  Mert Bora ALPER <bora@boramalper.org>.")
	zap.L().Info("Dedicated to Cemile Binay, in whose hands I thrived.")

	// opFlags is the "operational flags"
	opFlags := parseFlags()

	switch opFlags.Verbosity {
	case 0:
		loggerLevel.SetLevel(zap.WarnLevel)
	case 1:
		loggerLevel.SetLevel(zap.InfoLevel)
		// Default: i.e. in case of 2 or more.
	default:
		loggerLevel.SetLevel(zap.DebugLevel)
	}

	zap.ReplaceGlobals(logger)

	// Handle Ctrl-C gracefully.
	interruptChan := make(chan os.Signal)
	signal.Notify(interruptChan, os.Interrupt)

	database, err := persistence.MakeDatabase(opFlags.DatabaseURL)
	if err != nil {
		logger.Sugar().Fatalf("Could not open the database at `%s`: %s", opFlags.DatabaseURL, err.Error())
	}

	trawlingManager       := dht.NewTrawlingManager(opFlags.TrawlerMlAddrs)
	metadataSink          := bittorrent.NewMetadataSink(opFlags.FetcherAddr)
	completingCoordinator := NewCompletingCoordinator(database, CompletingCoordinatorOpFlags{
		LeechClAddr:   opFlags.LeechClAddr,
		LeechMlAddr:   opFlags.LeechMlAddr,
		LeechTimeout:  opFlags.LeechTimeout,
		ReadmeMaxSize: opFlags.ReadmeMaxSize,
		ReadmeRegex:   opFlags.ReadmeRegex,
	})
	/*
	refreshingCoordinator := NewRefreshingCoordinator(database, RefreshingCoordinatorOpFlags{

	})
	*/

	for {
		select {
		case result := <-trawlingManager.Output():
			logger.Debug("result: ", zap.String("hash", result.InfoHash.String()))
			exists, err := database.DoesTorrentExist(result.InfoHash[:])
			if err != nil {
				zap.L().Fatal("Could not check whether torrent exists!", zap.Error(err))
			} else if !exists {
				metadataSink.Sink(result)
			}

		case metadata := <-metadataSink.Drain():
			if err := database.AddNewTorrent(metadata.InfoHash, metadata.Name, metadata.Files); err != nil {
				logger.Sugar().Fatalf("Could not add new torrent %x to the database: %s",
					metadata.InfoHash, err.Error())
			}
			logger.Sugar().Infof("D I S C O V E R E D: `%s` %x", metadata.Name, metadata.InfoHash)

			if readmePath := findReadme(opFlags.ReadmeRegex, metadata.Files); readmePath != nil {
				completingCoordinator.Request(metadata.InfoHash, *readmePath, metadata.Peers)
			}

		case result := <-completingCoordinator.Output():
			database.AddReadme(result.InfoHash, result.Path, result.Data)

		case <-interruptChan:
			trawlingManager.Terminate()
			break
		}
	}
}

func parseFlags() (opF opFlags) {
	var cmdF cmdFlags

	_, err := flags.Parse(&cmdF)
	if err != nil {
		zap.S().Fatalf("Could not parse command-line flags! %s", err.Error())
	}

	// TODO: Check Database URL here
	opF.DatabaseURL = cmdF.DatabaseURL

	if err = checkAddrs(cmdF.TrawlerMlAddrs); err != nil {
		zap.S().Fatalf("Of argument (list) `trawler-ml-addr` %s", err.Error())
	} else {
		opF.TrawlerMlAddrs = cmdF.TrawlerMlAddrs
	}

	if cmdF.TrawlerMlInterval <= 0 {
		zap.L().Fatal("Argument `trawler-ml-interval` must be greater than zero, if supplied.")
	} else {
		// 1 decisecond = 100 milliseconds = 0.1 seconds
		opF.TrawlerMlInterval = time.Duration(cmdF.TrawlerMlInterval) * 100 * time.Millisecond
	}

	if err = checkAddrs([]string{cmdF.FetcherAddr}); err != nil {
		zap.S().Fatalf("Of argument `fetcher-addr` %s", err.Error())
	} else {
		opF.FetcherAddr = cmdF.FetcherAddr
	}

	if cmdF.FetcherTimeout <= 0 {
		zap.L().Fatal("Argument `fetcher-timeout` must be greater than zero, if supplied.")
	} else {
		opF.FetcherTimeout = time.Duration(cmdF.FetcherTimeout) * time.Second
	}

	if err = checkAddrs(cmdF.StatistMlAddrs); err != nil {
		zap.S().Fatalf("Of argument (list) `statist-ml-addr` %s", err.Error())
	} else {
		opF.StatistMlAddrs = cmdF.StatistMlAddrs
	}

	if cmdF.StatistMlTimeout <= 0 {
		zap.L().Fatal("Argument `statist-ml-timeout` must be greater than zero, if supplied.")
	} else {
		opF.StatistMlTimeout = time.Duration(cmdF.StatistMlTimeout) * time.Second
	}

	if err = checkAddrs([]string{cmdF.LeechClAddr}); err != nil {
		zap.S().Fatalf("Of argument `leech-cl-addr` %s", err.Error())
	} else {
		opF.LeechClAddr = cmdF.LeechClAddr
	}

	if err = checkAddrs([]string{cmdF.LeechMlAddr}); err != nil {
		zap.S().Fatalf("Of argument `leech-ml-addr` %s", err.Error())
	} else {
		opF.LeechMlAddr = cmdF.LeechMlAddr
	}

	if cmdF.LeechTimeout <= 0 {
		zap.L().Fatal("Argument `leech-timeout` must be greater than zero, if supplied.")
	} else {
		opF.LeechTimeout = time.Duration(cmdF.LeechTimeout) * time.Second
	}

	if cmdF.ReadmeMaxSize <= 0 {
		zap.L().Fatal("Argument `readme-max-size` must be greater than zero, if supplied.")
	} else {
		opF.ReadmeMaxSize = cmdF.ReadmeMaxSize
	}

	opF.ReadmeRegex, err = regexp.Compile(cmdF.ReadmeRegex)
	if err != nil {
		zap.S().Fatalf("Argument `readme-regex` is not a valid regex: %s", err.Error())
	}

	opF.Verbosity = len(cmdF.Verbose)
	opF.Profile = cmdF.Profile

	return
}

func checkAddrs(addrs []string) error {
	for i, addr := range addrs {
		// We are using ResolveUDPAddr but it works equally well for checking TCPAddr(esses) as
		// well.
		_, err := net.ResolveUDPAddr("udp", addr)
		if err != nil {
			return fmt.Errorf("with %d(th) address `%s`: %s", i + 1, addr, err.Error())
		}
	}
	return nil
}

// findReadme looks for a possible Readme file whose path is matched by the pathRegex.
// If there are multiple matches, the first one is returned.
// If there are no matches, nil returned.
func findReadme(pathRegex *regexp.Regexp, files []persistence.File) *string {
	for _, file := range files {
		if pathRegex.MatchString(file.Path) {
			return &file.Path
		}
	}
	return nil
}
