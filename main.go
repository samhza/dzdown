package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/erebid/go-deezer/deezer"
)

func main() {
	var (
		arl   string
		maxDl int
	)
	flag.StringVar(&arl, "arl", "", "deezer arl for auth")
	flag.IntVar(&maxDl, "maxdl", 4, "max # of songs to download at a time")
	flag.Parse()
	if arl == "" {
		println("no arl token provided")
		os.Exit(1)
	}
	c, err := deezer.NewClient(arl)
	if err != nil {
		println("error creating client:", err.Error())
		os.Exit(1)
	}
	var songs []deezer.Song
	for _, link := range flag.Args() {
		ctype, id := deezer.ParseLink(link)
		if ctype == "" {
			println("Invalid link:", link)
			continue
		}
		switch ctype {
		case deezer.ContentSong:
			song, err := c.Song(id)
			if err != nil {
				println("failed to get song", err.Error())
				continue
			}
			songs = append(songs, song)
		case deezer.ContentAlbum:
			albsongs, err := c.SongsByAlbum(id, -1)
			if err != nil {
				println("failed to get album", err.Error())
				continue
			}
			songs = append(songs, albsongs...)
		case deezer.ContentArtist:
			albums, err := c.AlbumsByArtist(id)
			if err != nil {
				println("failed to get artist", err.Error())
				continue
			}
			for _, album := range albums {
				albsongs, err := c.SongsByAlbum(album.ID, -1)
				if err != nil {
					println("failed to get album", err.Error())
					continue
				}
				songs = append(songs, albsongs...)
			}
		}
	}
	downloadSongs(songs, c, maxDl)
}

func downloadSongs(songs []deezer.Song, c *deezer.Client, maxDl int) {
	failed := make(chan deezer.Song, len(songs))
	sem := make(chan int, maxDl)
	var wg sync.WaitGroup
	wg.Add(len(songs))
	for _, song := range songs {
		go downloadSong(c, song, &wg, sem, failed)
	}
	wg.Wait()
	close(failed)
	for failedsong := range failed {
		println("song failed to download:", failedsong.Title)
	}
}

func downloadSong(c *deezer.Client, song deezer.Song, wg *sync.WaitGroup, sem chan int, failed chan deezer.Song) {
	defer wg.Done()
	sem <- 1
	quality, err := deezer.ValidSongQuality(song, deezer.Quality(0))
	if err != nil {
		failed <- song
		<-sem
		return
	}
	url := deezer.SongDownloadURL(song, quality)
	sng, err := c.Get(url)
	if err != nil {
		failed <- song
		<-sem
		return
	}
	defer sng.Body.Close()
	file, err := os.Create(fmt.Sprintf("%s - %d %s.mp3", song.AlbumTitle, song.TrackNumber, song.Title))
	if err != nil {
		failed <- song
		<-sem
		return
	}
	defer file.Close()
	if err != nil {
		failed <- song
		<-sem
		return
	}
	reader, err := deezer.NewEncryptedSongReader(sng.Body, song)
	if err != nil {
		failed <- song
		<-sem
		return
	}
	io.Copy(file, reader)
	println("downloaded", song.Title)
	<-sem
}
