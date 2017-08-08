package bittorrent

import (
	"time"
	"strings"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
	"go.uber.org/zap"
)


func (ms *MetadataSink) awaitMetadata(infoHash metainfo.Hash, peer torrent.Peer) {
	zap.L().Sugar().Debugf("awaiting %x...", infoHash[:])
	t, isNew := ms.client.AddTorrentInfoHash(infoHash)
	t.AddPeers([]torrent.Peer{peer})
	if !isNew {
		// If the recently added torrent is not new, then quit as we do not want multiple
		// awaitMetadata goroutines waiting on the same torrent.
		return
	} else {
		defer t.Drop()
	}

	// Wait for the torrent client to receive the metadata for the torrent, meanwhile allowing
	// termination to be handled gracefully.
	select {
	case <- ms.termination:
		return

	case <- t.GotInfo():
	}
	zap.L().Sugar().Warnf("==== GOT INFO for %x", infoHash[:])

	info := t.Info()
	var files []metainfo.FileInfo
	if len(info.Files) == 0 {
		if strings.ContainsRune(info.Name, '/') {
			// A single file torrent cannot have any '/' characters in its name. We treat it as
			// illegal.
			return
		}
		files = []metainfo.FileInfo{{Length: info.Length, Path:[]string{info.Name}}}
	} else {
		// TODO: We have to make sure that anacrolix/torrent checks for '/' character in file paths
		// before concatenating them. This is currently assumed here. We should write a test for it.
		files = info.Files
	}

	var totalSize uint64
	for _, file := range files {
		if file.Length < 0 {
			// All files' sizes must be greater than or equal to zero, otherwise treat them as
			// illegal and ignore.
			return
		}
		totalSize += uint64(file.Length)
	}

	ms.flush(Metadata{
		InfoHash: infoHash[:],
		Name: info.Name,
		TotalSize: totalSize,
		DiscoveredOn: time.Now().Unix(),
		Files: files,
	})
}
