package main

import (
	"regexp"
	"sync"
	"time"

	"github.com/anacrolix/torrent"

	"persistence"

	"magneticod/bittorrent"

)

type completionRequest struct {
	infoHash []byte
	path     string
	peers    []torrent.Peer
	time     time.Time
}

type completionResult struct {
	InfoHash []byte
	Path     string
	Data     []byte
}

type CompletingCoordinator struct {
	database      persistence.Database
	maxReadmeSize uint
	sink          *bittorrent.FileSink
	queue         chan completionRequest
	queueMutex    sync.Mutex
	outputChan    chan completionResult
	readmeRegex   *regexp.Regexp
	terminated    bool
	termination   chan interface{}
}

type CompletingCoordinatorOpFlags struct {
	LeechClAddr   string
	LeechMlAddr   string
	LeechTimeout  time.Duration
	ReadmeMaxSize uint
	ReadmeRegex *regexp.Regexp
}

func NewCompletingCoordinator(database persistence.Database, opFlags CompletingCoordinatorOpFlags) (cc *CompletingCoordinator) {
	cc = new(CompletingCoordinator)
	cc.database = database
	cc.maxReadmeSize = opFlags.ReadmeMaxSize
	cc.sink = bittorrent.NewFileSink(opFlags.LeechClAddr, opFlags.LeechMlAddr, opFlags.LeechTimeout)
	cc.queue = make(chan completionRequest, 100)
	cc.readmeRegex = opFlags.ReadmeRegex
	cc.termination = make(chan interface{})
	return
}

func (cc *CompletingCoordinator) Request(infoHash []byte, path string, peers []torrent.Peer) {
	cc.queueMutex.Lock()
	defer cc.queueMutex.Unlock()

	// If queue is full discard the oldest request as it is more likely to be outdated.
	if len(cc.queue) == cap(cc.queue) {
		<- cc.queue
	}

	// Imagine, if this function [Request()] was called by another goroutine right when we were
	// here: the moment where we removed the oldest entry in the queue to free a single space for
	// the newest one. Imagine, now, that the second Request() call manages to add its own entry
	// to the queue, making the current goroutine wait until the cc.queue channel is available.
	//
	// Hence to prevent that we use cc.queueMutex

	cc.queue <- completionRequest{
		infoHash: infoHash,
		path: path,
		peers: peers,
		time: time.Now(),
	}
}

func (cc *CompletingCoordinator) Start() {
	go cc.complete()
}

func (cc *CompletingCoordinator) Output() <-chan completionResult {
	return cc.outputChan
}

func (cc *CompletingCoordinator) complete() {
	for {
		select {
		case request := <-cc.queue:
			// Discard requests older than 2 minutes.
			// TODO: Instead of settling on 2 minutes as an arbitrary value, do some research to
			//       learn average peer lifetime in the BitTorrent network.
			if time.Now().Sub(request.time) > 2 * time.Minute {
				continue
			}
			cc.sink.Sink(request.infoHash, request.path, request.peers)

		case <-cc.termination:
			break

		default:
			cc.database.FindAnIncompleteTorrent(cc.readmeRegex, cc.maxReadmeSize)
		}
	}
}
