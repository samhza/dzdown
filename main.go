package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/erebid/go-deezer/deezer"
)

func main() {
	var (
		arl     string
		quality string
		maxDl   int
	)
	flag.StringVar(&arl, "arl", "", "deezer arl for auth")
	flag.StringVar(&quality, "q", "mp3-320", "deezer arl for auth")
	flag.IntVar(&maxDl, "maxdl", 4, "max # of songs to download at a time")
	flag.Parse()
	if arl == "" {
		println("no arl token provided")
		os.Exit(1)
	}
	var preferredQuality deezer.Quality
	switch strings.ToLower(quality) {
	case "mp3-128":
		preferredQuality = deezer.MP3128
	case "mp3-320":
		preferredQuality = deezer.MP3320
	case "flac":
		preferredQuality = deezer.FLAC
	default:
		preferredQuality = deezer.MP3320
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
	downloadSongs(songs, c, maxDl, preferredQuality)
}

func downloadSongs(songs []deezer.Song, c *deezer.Client,
	maxDl int, qual deezer.Quality) {
	sem := make(chan int, maxDl)
	var wg sync.WaitGroup
	wg.Add(len(songs))
	for _, song := range songs {
		go downloadSong(c, song, qual, &wg, sem)
	}
	wg.Wait()
}

func downloadSong(c *deezer.Client, song deezer.Song,
	preferredQuality deezer.Quality, wg *sync.WaitGroup, sem chan int) {

	defer wg.Done()
	sem <- 1
	var body io.ReadCloser
	quality := preferredQuality
	for {
		url := deezer.SongDownloadURL(song, quality)
		sng, err := c.Get(url)
		if err != nil {
			println("error getting song", err)
			<-sem
			return
		}
		if sng.StatusCode == 200 {
			body = sng.Body
			defer body.Close()
			break
		} else {
			sng.Body.Close()
			qualities := c.AvailableQualities(song)
			if len(qualities) == 0 {
				println("song not available:", song.Title)
				<-sem
				return
			}
			for _, q := range qualities {
				if q == preferredQuality {
					quality = q
					continue
				}
			}
			quality = qualities[len(qualities)-1]
		}
	}
	filepath := fmt.Sprintf(
		"%s/%s/%d - %s.mp3",
		clean(song.ArtistName),
		clean(song.AlbumTitle),
		song.TrackNumber,
		clean(song.Title),
	)
	err := os.MkdirAll(path.Dir(filepath), 0755)
	if err != nil {
		println("failed to create directory for music", err)
		<-sem
		return
	}
	file, err := os.Create(filepath)
	if err != nil {
		println("failed to create file for song", err)
		<-sem
		return
	}
	reader, err := deezer.NewEncryptedSongReader(body, song.ID)
	if err != nil {
		println("failed to create encrypted reader for song", err)
		<-sem
		return
	}
	_, err = io.Copy(file, reader)
	if err != nil {
		println("failed to download song", err)
		<-sem
		return
	}
	println("downloaded", song.Title)
	<-sem
}

func clean(name string) (cleaned string) {
	cleaned = strings.Replace(name, string(filepath.Separator), "", -1)
	cleaned = strings.Replace(cleaned, string(filepath.ListSeparator), "", -1)
	return
}
