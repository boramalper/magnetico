package main

import (
	"net"
	"os"
	"os/signal"
	"regexp"

	"github.com/jessevdk/go-flags"
	"github.com/pkg/profile"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"magneticod/bittorrent"
	"magneticod/dht"
	"fmt"
	"time"
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
	ReadmeRegexes []string `long:"readme-regex" description:"Regular expression(s) which will be tested against the name of the README files, in the supplied order."`

	Verbose []bool `short:"v" long:"verbose" description:"Increases verbosity."`

	Profile string `long:"profile" description:"Enable profiling." default:""`

	// ==== Deprecated Flags ====
	// TODO: don't even support deprecated flags!

	// DatabaseFile is akin to Database flag, except that it was used when SQLite was the only
	// persistence backend ever conceived, so it's the path* to the database file, which was -by
	// default- located in wherever appdata module on Python said:
	//     On GNU/Linux    : `/home/<USER>/.local/share/magneticod/database.sqlite3`
	//     On Windows      : TODO?
	//     On MacOS (OS X) : TODO?
	//     On BSDs?        : TODO?
	//     On anywhere else: TODO?
	// TODO: Is the path* absolute or can be relative as well?
	// DatabaseFile string
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

	// TODO: is this even supported by anacrolix/torrent?
	FetcherAddr    string
	FetcherTimeout time.Duration

	StatistMlAddrs   []string
	StatistMlTimeout time.Duration

	// TODO: is this even supported by anacrolix/torrent?
	LeechClAddr   string
	LeechMlAddr   string
	LeechTimeout  time.Duration
	ReadmeMaxSize uint
	ReadmeRegexes []*regexp.Regexp

	Verbosity int

	Profile string
}

func main() {
	atom := zap.NewAtomicLevel()
	// Logging levels: ("debug", "info", "warn", "error", "dpanic", "panic", and "fatal").
	logger := zap.New(zapcore.NewCore(
		zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig()),
		zapcore.Lock(os.Stderr),
		atom,
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
		atom.SetLevel(zap.WarnLevel)
	case 1:
		atom.SetLevel(zap.InfoLevel)
	// Default: i.e. in case of 2 or more.
	default:
		atom.SetLevel(zap.DebugLevel)
	}

	zap.ReplaceGlobals(logger)

	/*
		updating_manager := nil
		statistics_sink := nil
		completing_manager := nil
		file_sink := nil
	*/
	// Handle Ctrl-C gracefully.
	interrupt_chan := make(chan os.Signal)
	signal.Notify(interrupt_chan, os.Interrupt)

	database, err := NewDatabase(opFlags.Database)
	if err != nil {
		logger.Sugar().Fatalf("Could not open the database at `%s`: %s", opFlags.Database, err.Error())
	}

	trawlingManager := dht.NewTrawlingManager(opFlags.MlTrawlerAddrs)
	metadataSink := bittorrent.NewMetadataSink(opFlags.FetcherAddr)
	fileSink := bittorrent.NewFileSink()

	go func() {
		for {
			select {
			case result := <-trawlingManager.Output():
				logger.Debug("result: ", zap.String("hash", result.InfoHash.String()))
				if !database.DoesExist(result.InfoHash[:]) {
					metadataSink.Sink(result)
				}

			case metadata := <-metadataSink.Drain():
				logger.Sugar().Infof("D I S C O V E R E D: `%s` %x",
					metadata.Name, metadata.InfoHash)
				if err := database.AddNewTorrent(metadata); err != nil {
					logger.Sugar().Fatalf("Could not add new torrent %x to the database: %s",
						metadata.InfoHash, err.Error())
				}

			case <-interrupt_chan:
				trawlingManager.Terminate()
				break
			}
		}
	}()

	go func() {

	}()

	go func() {

	}()

	/*
		for {
			select {

			case updating_manager.Output():

			case statistics_sink.Sink():

			case completing_manager.Output():

			case file_sink.Sink():
	*/

	<-interrupt_chan
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
		zap.S().Fatal("Of argument `leech-cl-addr` %s", err.Error())
	} else {
		opF.LeechClAddr = cmdF.LeechClAddr
	}

	if err = checkAddrs([]string{cmdF.LeechMlAddr}); err != nil {
		zap.S().Fatal("Of argument `leech-ml-addr` %s", err.Error())
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

	for i, s := range cmdF.ReadmeRegexes {
		regex, err := regexp.Compile(s)
		if err != nil {
			zap.S().Fatalf("Of argument `readme-regex` with %d(th) regex `%s`: %s", i + 1, s, err.Error())
		} else {
			opF.ReadmeRegexes = append(opF.ReadmeRegexes, regex)
		}
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
