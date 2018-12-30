package main

import (
	"math/rand"
	"net"
	"os"
	"os/signal"
	"runtime/pprof"
	"time"

	"github.com/pkg/errors"

	"github.com/jessevdk/go-flags"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/boramalper/magnetico/pkg/util"

	"github.com/boramalper/magnetico/cmd/magneticod/bittorrent/metadata"
	"github.com/boramalper/magnetico/cmd/magneticod/dht"

	"github.com/Wessie/appdirs"

	"github.com/boramalper/magnetico/pkg/persistence"
)

type opFlags struct {
	DatabaseURL string

	TrawlerMlAddrs    []string
	TrawlerMlInterval time.Duration

	LeechMaxN int

	Verbosity int
	Profile   string
}

var compiledOn string

func main() {
	loggerLevel := zap.NewAtomicLevel()
	// Logging levels: ("debug", "info", "warn", "error", "dpanic", "panic", and "fatal").
	logger := zap.New(zapcore.NewCore(
		zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig()),
		zapcore.Lock(os.Stderr),
		loggerLevel,
	))
	defer func() {
		if err := logger.Sync(); err != nil {
			panic(err)
		}
	}()
	zap.ReplaceGlobals(logger)

	// opFlags is the "operational flags"
	opFlags, err := parseFlags()
	if err != nil {
		// Do not print any error messages as jessevdk/go-flags already did.
		return
	}

	zap.L().Info("magneticod v0.7.0-beta2 has been started.")
	zap.L().Info("Copyright (C) 2018  Mert Bora ALPER <bora@boramalper.org>.")
	zap.L().Info("Dedicated to Cemile Binay, in whose hands I thrived.")
	zap.S().Infof("Compiled on %s", compiledOn)

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
			zap.L().Panic("Could not open the cpu profile file!", zap.Error(err))
		}
		if err = pprof.StartCPUProfile(file); err != nil {
			zap.L().Fatal("Could not start CPU profiling!", zap.Error(err))
		}
		defer func() {
			if err = file.Sync(); err != nil {
				zap.L().Fatal("Could not sync profiling file!", zap.Error(err))
			}
		}()
		defer func() {
			if err = file.Close(); err != nil {
				zap.L().Fatal("Could not close profiling file!", zap.Error(err))
			}
		}()
		defer pprof.StopCPUProfile()

	case "memory":
		zap.L().Panic("NOT IMPLEMENTED")

	case "trace":
		zap.L().Panic("NOT IMPLEMENTED")
	}

	// Initialise the random number generator
	rand.Seed(time.Now().UnixNano())

	// Handle Ctrl-C gracefully.
	interruptChan := make(chan os.Signal, 1)
	signal.Notify(interruptChan, os.Interrupt)

	database, err := persistence.MakeDatabase(opFlags.DatabaseURL, logger)
	if err != nil {
		logger.Sugar().Fatalf("Could not open the database at `%s`", opFlags.DatabaseURL, zap.Error(err))
	}

	trawlingManager := dht.NewTrawlingManager(opFlags.TrawlerMlAddrs, opFlags.TrawlerMlInterval)
	metadataSink := metadata.NewSink(2*time.Minute, opFlags.LeechMaxN)

	zap.L().Debug("Peer ID", zap.ByteString("peerID", metadataSink.PeerID))

	// The Event Loop
	for stopped := false; !stopped; {
		select {
		case result := <-trawlingManager.Output():
			zap.L().Debug("Trawled!", util.HexField("infoHash", result.InfoHash[:]))
			exists, err := database.DoesTorrentExist(result.InfoHash[:])
			if err != nil {
				zap.L().Fatal("Could not check whether torrent exists!", zap.Error(err))
			} else if !exists {
				metadataSink.Sink(result)
			}

		case md := <-metadataSink.Drain():
			if err := database.AddNewTorrent(md.InfoHash, md.Name, md.Files); err != nil {
				zap.L().Fatal("Could not add new torrent to the database",
					util.HexField("infohash", md.InfoHash), zap.Error(err))
			}
			zap.L().Info("Fetched!", zap.String("name", md.Name), util.HexField("infoHash", md.InfoHash))

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
	var cmdF struct {
		DatabaseURL string `long:"database" description:"URL of the database."`

		TrawlerMlAddrs    []string `long:"trawler-ml-addr" description:"Address(es) to be used by trawling DHT (Mainline) nodes." default:"0.0.0.0:0"`
		TrawlerMlInterval uint     `long:"trawler-ml-interval" description:"Trawling interval in integer seconds."`

		LeechMaxN uint `long:"leech-max-n" description:"Maximum number of leeches." default:"1000"`

		Verbose []bool `short:"v" long:"verbose" description:"Increases verbosity."`
		Profile string `long:"profile" description:"Enable profiling." choice:"cpu" choice:"memory" choice:"trace"`
	}

	opF := new(opFlags)

	_, err := flags.Parse(&cmdF)
	if err != nil {
		return nil, err
	}

	if cmdF.DatabaseURL == "" {
		opF.DatabaseURL =
			"sqlite3://" +
				appdirs.UserDataDir("magneticod", "", "", false) +
				"/database.sqlite3" +
				"?_journal_mode=WAL" + // https://github.com/mattn/go-sqlite3#connection-string
				"&_busy_timeout=3000" + // in milliseconds
				"&_foreign_keys=true"

	} else {
		opF.DatabaseURL = cmdF.DatabaseURL
	}

	if err = checkAddrs(cmdF.TrawlerMlAddrs); err != nil {
		zap.S().Fatalf("Of argument (list) `trawler-ml-addr`", zap.Error(err))
	} else {
		opF.TrawlerMlAddrs = cmdF.TrawlerMlAddrs
	}

	if cmdF.TrawlerMlInterval == 0 {
		opF.TrawlerMlInterval = 1 * time.Second
	} else {
		opF.TrawlerMlInterval = time.Duration(cmdF.TrawlerMlInterval) * time.Second
	}

	opF.LeechMaxN = int(cmdF.LeechMaxN)
	if opF.LeechMaxN > 1000 {
		zap.S().Warnf(
			"Beware that on many systems max # of file descriptors per process is limited to 1024. " +
				"Setting maximum number of leeches greater than 1k might cause \"too many open files\" errors!",
		)
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
			return errors.Wrapf(err, "%d(th) address (%s) error", i+1, addr)
		}
	}
	return nil
}
