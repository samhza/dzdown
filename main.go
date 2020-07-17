package main

import (
	"flag"
	"fmt"
	"io"
	"log"
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
		log.Fatalln("no arl token provided")
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
		log.Fatalln("error creating client:", err.Error())
	}
	var songs []deezer.Song
	for _, link := range flag.Args() {
		ctype, id := deezer.ParseLink(link)
		if ctype == "" {
			log.Println("Invalid link:", link)
			continue
		}
		switch ctype {
		case deezer.ContentSong:
			song, err := c.Song(id)
			if err != nil {
				log.Println("failed to get song", err.Error())
				continue
			}
			songs = append(songs, *song)
		case deezer.ContentAlbum:
			albsongs, err := c.SongsByAlbum(id, -1)
			if err != nil {
				log.Println("failed to get album", err.Error())
				continue
			}
			songs = append(songs, albsongs...)
		case deezer.ContentArtist:
			albums, err := c.AlbumsByArtist(id)
			if err != nil {
				log.Println("failed to get artist", err.Error())
				continue
			}
			for _, album := range albums {
				albsongs, err := c.SongsByAlbum(album.ID, -1)
				if err != nil {
					log.Println("failed to get album", err.Error())
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
			log.Println("error getting song", err)
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
				log.Println("song not available:", song.Title)
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
		"%s/%s/%d - %s.%s",
		clean(song.ArtistName),
		clean(song.AlbumTitle),
		song.TrackNumber,
		clean(song.Title),
		ext(quality),
	)
	err := os.MkdirAll(path.Dir(filepath), 0755)
	if err != nil {
		log.Println("failed to create directory for music", err)
		<-sem
		return
	}
	file, err := os.Create(filepath)
	defer file.Close()
	if err != nil {
		log.Println("failed to create file for song", err)
		<-sem
		return
	}
	if deezer.FLAC != quality {
		tagMP3(c, file, song)
	}
	reader, err := deezer.NewEncryptedSongReader(body, song.ID)
	if err != nil {
		log.Println("failed to create encrypted reader for song", err)
		<-sem
		return
	}
	_, err = io.Copy(file, reader)
	if err != nil {
		log.Println("failed to download song", err)
		<-sem
		return
	}
	fmt.Println(deezer.Link(deezer.ContentSong, song.ID))
	<-sem
}

func clean(name string) (cleaned string) {
	cleaned = strings.Replace(name, string(filepath.Separator), "", -1)
	cleaned = strings.Replace(cleaned, string(filepath.ListSeparator), "", -1)
	return
}

func ext(q deezer.Quality) string {
	if q == deezer.FLAC {
		return "flac"
	}
	return "mp3"
}
