package bittorrent

import (
	"time"
	"strings"

	"github.com/anacrolix/missinggo"
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
		// Return immediately if we are trying to await on an ongoing metadata-fetching operation.
		// Each ongoing operation should have one and only one "await*" function waiting on it.
		return
	}

	// Wait for the torrent client to receive the metadata for the torrent, meanwhile allowing
	// termination to be handled gracefully.
	var info *metainfo.Info
	select {
	case <- t.GotInfo():
		info = t.Info()
		t.Drop()

	case <-time.After(5 * time.Minute):
		zap.L().Sugar().Debugf("Fetcher timeout!  %x", infoHash)
		return

	case <- ms.termination:
		return
	}

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


func (fs *FileSink) awaitFile(infoHash []byte, filePath string, peer *torrent.Peer) {
	var infoHash_ [20]byte
	copy(infoHash_[:], infoHash)
	t, isNew := fs.client.AddTorrentInfoHash(infoHash_)
	if peer != nil {
		t.AddPeers([]torrent.Peer{*peer})
	}
	if !isNew {
		// Return immediately if we are trying to await on an ongoing file-downloading operation.
		// Each ongoing operation should have one and only one "await*" function waiting on it.
		return
	}

	// Setup & start the timeout timer.
	timeout := time.After(fs.timeout)

	// Once we return from this function, drop the torrent from the client.
	// TODO: Check if dropping a torrent also cancels any outstanding read operations?
	defer t.Drop()

	select {
	case <-t.GotInfo():

	case <- timeout:
		return
	}

	var match *torrent.File
	for _, file := range t.Files() {
		if file.Path() == filePath {
			match = &file
		} else {
			file.Cancel()
		}
	}
	if match == nil {
		var filePaths []string
		for _, file := range t.Files() { filePaths = append(filePaths, file.Path()) }

		zap.L().Warn(
			"The leech (FileSink) has been requested to download a file which does not exist!",
			zap.ByteString("torrent", infoHash),
			zap.String("requestedFile", filePath),
			zap.Strings("allFiles", filePaths),
		)
	}


	reader := t.NewReader()
	defer reader.Close()

	fileDataChan := make(chan []byte)
	go downloadFile(*match, reader, fileDataChan)

	select {
	case fileData := <-fileDataChan:
		if fileData != nil {
			fs.flush(File{
				torrentInfoHash: infoHash,
				path: match.Path(),
				data: fileData,
			})
		}

	case <- timeout:
		zap.L().Debug(
			"Timeout while downloading a file!",
			zap.ByteString("torrent", infoHash),
			zap.String("file", filePath),
		)
	}
}


func downloadFile(file torrent.File, reader *torrent.Reader, fileDataChan chan<- []byte) {
	readSeeker := missinggo.NewSectionReadSeeker(reader, file.Offset(), file.Length())

	fileData := make([]byte, file.Length())
	n, err := readSeeker.Read(fileData)
	if int64(n) != file.Length() {
		zap.L().Debug(
			"Not all of a file could be read!",
			zap.ByteString("torrent", file.Torrent().InfoHash()[:]),
			zap.String("file", file.Path()),
			zap.Int64("fileLength", file.Length()),
			zap.Int("n", n),
		)
		fileDataChan <- nil
		return
	}
	if err != nil {
		zap.L().Debug(
			"Error while downloading a file!",
			zap.Error(err),
			zap.ByteString("torrent", file.Torrent().InfoHash()[:]),
			zap.String("file", file.Path()),
			zap.Int64("fileLength", file.Length()),
			zap.Int("n", n),
		)
		fileDataChan <- nil
		return
	}

	fileDataChan <- fileData
}

