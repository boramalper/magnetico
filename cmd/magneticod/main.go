package main

import (
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path"
	"runtime/pprof"
	"time"

	"github.com/jessevdk/go-flags"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/boramalper/magnetico/cmd/magneticod/bittorrent"
	"github.com/boramalper/magnetico/cmd/magneticod/dht"

	"github.com/Wessie/appdirs"
	"github.com/boramalper/magnetico/pkg/persistence"
	"net/url"
)

type cmdFlags struct {
	DatabaseURL string   `short:"d" long:"database" description:"URL of the database." env:"DATABASE"`
	BindAddr    []string `short:"b" long:"bind" description:"Address(es) that the Crawler should listen on." env:"BIND_ADDR" env-delim:"," default:"0.0.0.0:0"`
	Interval    uint     `short:"i" long:"interval" description:"Trawling Interval in milliseconds" env:"INTERVAL" default:"100"`
	Verbose     []bool   `short:"v" long:"verbose" description:"Increase verbosity"`
	Profile     string   `short:"p" long:"profile" description:"Enable profiling." choice:"cpu" choice:"memory" choice:"trace"`
}

type opFlags struct {
	DatabaseURL *url.URL
	BindAddr    []*net.UDPAddr
	Interval    time.Duration
	Verbosity   int
	Profile     string
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
	opFlags := parseFlags()

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
		file, err := os.OpenFile("magneticod_cpu.prof", os.O_CREATE|os.O_WRONLY, 0755)
		if err != nil {
			zap.L().Fatal("Could not open the cpu profile file!", zap.Error(err))
		}
		pprof.StartCPUProfile(file)
		defer file.Close()
		defer pprof.StopCPUProfile()

	case "memory":
		zap.L().Fatal("Memory profiling NOT IMPLEMENTED")

	case "trace":
		zap.L().Fatal("trace NOT IMPLEMENTED")
	}

	// Handle Ctrl-C gracefully.
	interruptChan := make(chan os.Signal)
	signal.Notify(interruptChan, os.Interrupt)

	database, err := persistence.MakeDatabase(opFlags.DatabaseURL, logger)
	if err != nil {
		logger.Sugar().Fatalf("Could not open the database at `%s`: %s", opFlags.DatabaseURL, err.Error())
	}

	trawlingManager := dht.NewTrawlingManager(opFlags.BindAddr)
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

func parseFlags() *opFlags {
	opF := new(opFlags)
	cmdF := new(cmdFlags)

	_, err := flags.Parse(cmdF)
	if err != nil {
		// Do not print any error messages as jessevdk/go-flags already did.
		os.Exit(1)
	}

	if cmdF.DatabaseURL == "" {
		cmdF.DatabaseURL = "sqlite3://" + path.Join(
			appdirs.UserDataDir("magneticod", "", "", false),
			"database.sqlite3",
		)
	}
	opF.DatabaseURL, err = url.Parse(cmdF.DatabaseURL)
	if err != nil {
		zap.L().Fatal("Failed to parse DB URL", zap.Error(err))
	}

	for _, addr := range cmdF.BindAddr {
		udpAddr, err := net.ResolveUDPAddr("udp", addr)
		if err != nil {
			zap.L().Fatal("Failed to parse Address", zap.Error(err))
		}
		opF.BindAddr = append(opF.BindAddr, udpAddr)
	}

	opF.Interval = time.Duration(cmdF.Interval) * time.Millisecond

	opF.Verbosity = len(cmdF.Verbose)

	opF.Profile = cmdF.Profile

	return opF
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
