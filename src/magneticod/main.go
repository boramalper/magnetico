package main

import (
	"net"
	"os"
	"os/signal"

	"go.uber.org/zap"
	"github.com/jessevdk/go-flags"

//	"magneticod/bittorrent"
	"magneticod/dht"
	"go.uber.org/zap/zapcore"
	"regexp"
	"magneticod/bittorrent"
)


type cmdFlags struct {
	Database string  `long:"database" description:"URL of the database."`

	MlTrawlerAddrs []string  `long:"ml-trawler-addrs" description:"Address(es) to be used by trawling DHT (Mainline) nodes." default:"0.0.0.0:0"`
	TrawlingInterval uint  `long:"trawling-interval" description:"Trawling interval in integer seconds."`

	// TODO: is this even supported by anacrolix/torrent?
	FetcherAddr string  `long:"fetcher-addr" description:"Address(es) to be used by ephemeral peers fetching torrent metadata." default:"0.0.0.0:0"`
	FetcherTimeout uint  `long:"fetcher-timeout" description:"Number of integer seconds before a fetcher timeouts."`
	// TODO: is this even supported by anacrolix/torrent?
	MaxMetadataSize uint  `long:"max-metadata-size" description:"Maximum metadata size -which must be greater than zero- in bytes."`

	MlStatisticianAddrs []string  `long:"ml-statistician-addrs" description:"Address(es) to be used by ephemeral nodes fetching latest statistics about individual torrents." default:"0.0.0.0:0"`
	StatisticianTimeout uint  `long:"statistician-timeout" description:"Number of integer seconds before a statistician timeouts."`

	// TODO: is this even supported by anacrolix/torrent?
	LeechAddr string  `long:"leech-addr" description:"Address(es) to be used by ephemeral peers fetching README files." default:"0.0.0.0:0"`
	LeechTimeout uint  `long:"leech-timeout" description:"Number of integer seconds before a leech timeouts."`
	MaxDescriptionSize uint  `long:"max-description-size" description:"Maximum size -which must be greater than zero- of a description file in bytes"`
	DescriptionNames []string `long:"description-names" description:"Regular expression(s) which will be tested against the name of the description files, in the supplied order."`

	Verbose []bool  `short:"v" long:"verbose" description:"Increases verbosity."`

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


type opFlags struct {
	Database string

	MlTrawlerAddrs []net.UDPAddr
	TrawlingInterval uint

	FetcherAddr net.TCPAddr
	FetcherTimeout uint
	// TODO: is this even supported by anacrolix/torrent?
	MaxMetadataSize uint

	MlStatisticianAddrs []net.UDPAddr
	StatisticianTimeout uint

	LeechAddr net.TCPAddr
	LeechTimeout uint
	MaxDescriptionSize uint
	DescriptionNames []regexp.Regexp

	Verbosity uint
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

	zap.L().Info("magneticod v0.7.0 has been started.")
	zap.L().Info("Copyright (C) 2017  Mert Bora ALPER <bora@boramalper.org>.")
	zap.L().Info("Dedicated to Cemile Binay, in whose hands I thrived.")

	// opFlags is the "operational flags"
	opFlags := parseFlags()

	logger.Sugar().Warn(">>>", opFlags.MlTrawlerAddrs)

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

	go func() {
		trawlingManager := dht.NewTrawlingManager(opFlags.MlTrawlerAddrs)
		metadataSink := bittorrent.NewMetadataSink(opFlags.FetcherAddr)

		for {
			select {
			case result := <-trawlingManager.Output():
				logger.Info("result: ", zap.String("hash", result.InfoHash.String()))
				metadataSink.Sink(result)

			case metadata := <-metadataSink.Drain():
				if err := database.AddNewTorrent(metadata); err != nil {
					logger.Sugar().Fatalf("Could not add new torrent %x to the database: %s", metadata.InfoHash, err.Error())
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


func parseFlags() (opFlags) {
	var cmdF cmdFlags

	_, err := flags.Parse(&cmdF)
	if err != nil {
		zap.L().Fatal("Error while parsing command-line flags: ", zap.Error(err))
	}

	mlTrawlerAddrs, err := hostPortsToUDPAddrs(cmdF.MlTrawlerAddrs)
	if err != nil {
		zap.L().Fatal("Erroneous ml-trawler-addrs argument supplied: ", zap.Error(err))
	}

	fetcherAddr, err := hostPortsToTCPAddr(cmdF.FetcherAddr)
	if err != nil {
		zap.L().Fatal("Erroneous fetcher-addr argument supplied: ", zap.Error(err))
	}

	mlStatisticianAddrs, err := hostPortsToUDPAddrs(cmdF.MlStatisticianAddrs)
	if err != nil {
		zap.L().Fatal("Erroneous ml-statistician-addrs argument supplied: ", zap.Error(err))
	}

	leechAddr, err := hostPortsToTCPAddr(cmdF.LeechAddr)
	if err != nil {
		zap.L().Fatal("Erroneous leech-addrs argument supplied: ", zap.Error(err))
	}

	var descriptionNames []regexp.Regexp
	for _, expr := range cmdF.DescriptionNames {
		regex, err := regexp.Compile(expr)
		if err != nil {
			zap.L().Fatal("Erroneous description-names argument supplied: ", zap.Error(err))
		}
		descriptionNames = append(descriptionNames, *regex)
	}


	opF := opFlags{
		Database: cmdF.Database,

		MlTrawlerAddrs: mlTrawlerAddrs,
		TrawlingInterval: cmdF.TrawlingInterval,

		FetcherAddr: fetcherAddr,
		FetcherTimeout: cmdF.FetcherTimeout,
		MaxMetadataSize: cmdF.MaxMetadataSize,

		MlStatisticianAddrs: mlStatisticianAddrs,
		StatisticianTimeout: cmdF.StatisticianTimeout,

		LeechAddr: leechAddr,
		LeechTimeout: cmdF.LeechTimeout,
		MaxDescriptionSize: cmdF.MaxDescriptionSize,
		DescriptionNames: descriptionNames,

		Verbosity: uint(len(cmdF.Verbose)),
	}

	return opF
}


func hostPortsToUDPAddrs(hostport []string) ([]net.UDPAddr, error) {
	udpAddrs := make([]net.UDPAddr, len(hostport))

	for i, hp := range hostport {
		udpAddr, err := net.ResolveUDPAddr("udp", hp)
		if err != nil {
			return nil, err
		}
		udpAddrs[i] = *udpAddr
	}

	return udpAddrs, nil
}


func hostPortsToTCPAddr(hostport string) (net.TCPAddr, error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", hostport)
	if err != nil {
		return net.TCPAddr{}, err
	}

	return *tcpAddr, nil
}
