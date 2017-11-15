package main

import (
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"os/signal"
	"time"

	"github.com/jessevdk/go-flags"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"magnetico/magneticod/bittorrent"
	"magnetico/magneticod/dht"

	"magnetico/persistence"
	"runtime/pprof"
	"github.com/Wessie/appdirs"
	"path"
)

type cmdFlags struct {
	DatabaseURL string `long:"database" description:"URL of the database."`

	TrawlerMlAddrs    []string `long:"trawler-ml-addr" description:"Address(es) to be used by trawling DHT (Mainline) nodes." default:"0.0.0.0:0"`
	TrawlerMlInterval uint     `long:"trawler-ml-interval" description:"Trawling interval in integer deciseconds (one tenth of a second)."`

	Verbose []bool `short:"v" long:"verbose" description:"Increases verbosity."`
	Profile string `long:"profile" description:"Enable profiling." choice:"cpu" choice:"memory" choice:"trace"`
}

type opFlags struct {
	DatabaseURL string

	TrawlerMlAddrs    []string
	TrawlerMlInterval time.Duration

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

	// opFlags is the "operational flags"
	opFlags, err := parseFlags()
	if err != nil {
		// Do not print any error messages as jessevdk/go-flags already did.
		return
	}

	zap.L().Info("magneticod v0.7.0 has been started.")
	zap.L().Info("Copyright (C) 2017  Mert Bora ALPER <bora@boramalper.org>.")
	zap.L().Info("Dedicated to Cemile Binay, in whose hands I thrived.")

	switch opFlags.Verbosity {
	case 0:
		loggerLevel.SetLevel(zap.WarnLevel)
	case 1:
		loggerLevel.SetLevel(zap.InfoLevel)
	default: // Default: i.e. in case of 2 or more.
		// TODO: print the caller (function)'s name and line number!
		loggerLevel.SetLevel(zap.DebugLevel)
	}

	zap.ReplaceGlobals(logger)

	switch opFlags.Profile {
	case "cpu":
		file, err := os.OpenFile("magneticod_cpu.prof", os.O_CREATE | os.O_WRONLY, 0755)
		if err != nil {
			zap.L().Panic("Could not open the cpu profile file!", zap.Error(err))
		}
		pprof.StartCPUProfile(file)
		defer file.Close()
		defer pprof.StopCPUProfile()

	case "memory":
		zap.L().Panic("NOT IMPLEMENTED")

	case "trace":
		zap.L().Panic("NOT IMPLEMENTED")
	}

	// Handle Ctrl-C gracefully.
	interruptChan := make(chan os.Signal)
	signal.Notify(interruptChan, os.Interrupt)

	database, err := persistence.MakeDatabase(opFlags.DatabaseURL, false, logger)
	if err != nil {
		logger.Sugar().Fatalf("Could not open the database at `%s`: %s", opFlags.DatabaseURL, err.Error())
	}

	trawlingManager := dht.NewTrawlingManager(opFlags.TrawlerMlAddrs)
	metadataSink := bittorrent.NewMetadataSink(2 * time.Minute)

	// The Event Loop
	for stopped := false; !stopped; {
		select {
		case result := <-trawlingManager.Output():
			zap.L().Info("Trawled!", zap.String("infoHash", result.InfoHash.String()))
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
			zap.L().Info("Fetched!", zap.String("name", metadata.Name), zap.String("infoHash", hex.EncodeToString(metadata.InfoHash)))

		case <-interruptChan:
			trawlingManager.Terminate()
			stopped = true
		}
	}

	if err = database.Close(); err != nil {
		zap.L().Error("Could not close database!", zap.Error(err))
	}
}

func parseFlags() (*opFlags, error) {
	opF := new(opFlags)
	cmdF := new(cmdFlags)

	_, err := flags.Parse(cmdF)
	if err != nil {
		return nil, err
	}

	if cmdF.DatabaseURL == "" {
		opF.DatabaseURL = "sqlite3://" + path.Join(
				appdirs.UserDataDir("magneticod", "", "", false),
				"database.sqlite3",
			)
	} else {
		opF.DatabaseURL = cmdF.DatabaseURL
	}

	if err = checkAddrs(cmdF.TrawlerMlAddrs); err != nil {
		zap.S().Fatalf("Of argument (list) `trawler-ml-addr` %s", err.Error())
	} else {
		opF.TrawlerMlAddrs = cmdF.TrawlerMlAddrs
	}

	// 1 decisecond = 100 milliseconds = 0.1 seconds
	if cmdF.TrawlerMlInterval == 0 {
		opF.TrawlerMlInterval = time.Duration(1) * 100 * time.Millisecond
	} else {
		opF.TrawlerMlInterval = time.Duration(cmdF.TrawlerMlInterval) * 100 * time.Millisecond
	}

	opF.Verbosity = len(cmdF.Verbose)

	opF.Profile = cmdF.Profile

	return opF, nil
}

func checkAddrs(addrs []string) error {
	for i, addr := range addrs {
		// We are using ResolveUDPAddr but it works equally well for checking TCPAddr(esses) as
		// well.
		_, err := net.ResolveUDPAddr("udp", addr)
		if err != nil {
			return fmt.Errorf("with %d(th) address `%s`: %s", i+1, addr, err.Error())
		}
	}
	return nil
}
