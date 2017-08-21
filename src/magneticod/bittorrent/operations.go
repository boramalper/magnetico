package bittorrent

import (
	"time"
	"strings"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
	"go.uber.org/zap"
)


func (ms *MetadataSink) awaitMetadata(infoHash metainfo.Hash, peer torrent.Peer) {
	t, isNew := ms.client.AddTorrentInfoHash(infoHash)
	// If the infoHash we added was not new (i.e. it's already being downloaded by the client)
	// then t is the handle of the (old) torrent. We add the (presumably new) peer to the torrent
	// so we can increase the chance of operation being successful, or that the metadata might be
	// fetched.
	t.AddPeers([]torrent.Peer{peer})
	if !isNew {
		// If the recently added torrent is not new, then quit as we do not want multiple
		// awaitMetadata goroutines waiting on the same torrent.
		return
	} else {
		// Drop the torrent once we return from this function, whether we got the metadata or an
		// error.
		defer t.Drop()
	}

	// Wait for the torrent client to receive the metadata for the torrent, meanwhile allowing
	// termination to be handled gracefully.
	select {
	case <- t.GotInfo():

	case <-time.After(5 * time.Minute):
		zap.L().Sugar().Debugf("Fetcher timeout!  %x", infoHash)
		return

	case <- ms.termination:
		return
	}

	info := t.Info()
	var files []metainfo.FileInfo
	if len(info.Files) == 0 {
		if strings.ContainsRune(info.Name, '/') {
			// A single file torrent cannot have any '/' characters in its name. We treat it as
			// illegal.
			zap.L().Sugar().Debugf("!!!! illegal character in name!  \"%s\"", info.Name)
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
			zap.L().Sugar().Debugf("!!!! file size zero or less!  \"%s\" (%d)", file.Path, file.Length)
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
