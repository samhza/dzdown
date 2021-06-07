package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/godeezer/lib/deezer"
	"golang.org/x/sync/semaphore"
)

type dzdown struct {
	*deezer.Client
	preferredQuality deezer.Quality
	preferEdited     bool
	artSize          int
	maxDl            int
}

func main() {
	var (
		arl     string
		quality string
	)
	dz := &dzdown{}
	flag.StringVar(&arl, "arl", "", "deezer arl for auth")
	flag.StringVar(&quality, "q", "mp3-320", "deezer arl for auth")
	flag.IntVar(&dz.maxDl, "dl-limit", 4, "max # of songs to download at a time")
	flag.IntVar(&dz.artSize, "art-size", 800, "width/height of album art to download (max = 800)")
	flag.BoolVar(&dz.preferEdited, "prefer-edited", false,
		"whether to prefer edited/clean versions of albums over their unedited/explicit counterparts")
	flag.Parse()
	if arl == "" {
		log.Fatalln("no arl token provided")
	}
	switch strings.ToLower(quality) {
	case "mp3-128":
		dz.preferredQuality = deezer.MP3128
	case "mp3-320":
		dz.preferredQuality = deezer.MP3320
	case "flac":
		dz.preferredQuality = deezer.FLAC
	default:
		log.Fatalln("unknown quality:", strings.ToLower(quality))
	}
	dz.Client = deezer.NewClient(arl)
	var songs []deezer.Song
	for _, link := range flag.Args() {
		ctype, id := deezer.ParseURL(link)
		if ctype == "" {
			log.Println("Invalid link:", link)
			continue
		}
		switch ctype {
		case deezer.ContentSong:
			song, err := dz.Song(id)
			if err != nil {
				log.Println("failed to get song", err.Error())
				continue
			}
			songs = append(songs, *song)
		case deezer.ContentAlbum:
			albsongs, err := dz.SongsByAlbum(id, -1)
			if err != nil {
				log.Println("failed to get album", err.Error())
				continue
			}
			songs = append(songs, albsongs...)
		case deezer.ContentArtist:
			albums, err := dz.AlbumsByArtist(id)
			if err != nil {
				log.Println("failed to get artist", err.Error())
				continue
			}

			// There may be multiple "albums" with the same name fetched, some being
			// explicit and others being edited.
			var uniqueAlbums []deezer.Album
			for _, album := range albums {
				collisionIndex := -1
				for j, a := range uniqueAlbums {
					if album.Title == a.Title {
						collisionIndex = j
					}
				}
				if collisionIndex == -1 {
					uniqueAlbums = append(uniqueAlbums, album)
				} else {
					collisionStatus := uniqueAlbums[collisionIndex].ExplicitContent.LyricsStatus
					collisionIsEdited := collisionStatus == 3
					if dz.preferEdited != collisionIsEdited {
						uniqueAlbums[collisionIndex] = album
					}
				}
			}
			println(len(albums))
			println("filtered", len(uniqueAlbums))

			for _, album := range uniqueAlbums {
				albsongs, err := dz.SongsByAlbum(album.ID, -1)
				if err != nil {
					log.Println("failed to get album", err.Error())
					continue
				}
				songs = append(songs, albsongs...)
			}
		}
	}
	dz.downloadSongs(songs)
}

func (dz *dzdown) downloadSongs(songs []deezer.Song) {
	sem := semaphore.NewWeighted(int64(dz.maxDl))
	for _, song := range songs {
		if err := sem.Acquire(context.Background(), 1); err != nil {
			log.Printf("failed to acquire semaphore: %v\n", err)
		}
		go func(song deezer.Song) {
			defer sem.Release(1)
			dz.downloadSong(song)
		}(song)
	}
	if err := sem.Acquire(context.Background(), int64(dz.maxDl)); err != nil {
		log.Printf("failed to acquire semaphore: %v\n", err)
	}
}

func (dz *dzdown) downloadSong(song deezer.Song) {
	filepath := songFilepath(song, dz.preferredQuality)
	_, err := os.Stat(filepath)
	if !os.IsNotExist(err) {
		fmt.Println("file already exists, skipping download:", filepath)
		return
	}
	err = os.MkdirAll(path.Dir(filepath), 0755)
	if err != nil {
		log.Println("failed to create directory for music", err)
		return
	}
	err = dz.downloadCover(song)
	if err != nil {
		log.Println("error downloading album art:", err)
		return
	}
	var body io.ReadCloser
	quality := dz.preferredQuality
	for {
		url := song.DownloadURL(quality)
		sng, err := dz.Get(url)
		if err != nil {
			log.Println("error getting song", err)
			return
		}
		if sng.StatusCode == 200 {
			body = sng.Body
			defer body.Close()
			break
		} else {
			sng.Body.Close()
			qualities := dz.AvailableQualities(song)
			if len(qualities) == 0 {
				log.Println("song not available:", song.Title)
				return
			}
			quality = qualities[len(qualities)-1]
			filepath = songFilepath(song, quality)
		}
	}

	file, err := os.Create(filepath)
	if err != nil {
		log.Println("failed to create file for song", err)
		return
	}
	defer file.Close()

	reader, err := deezer.NewDecryptingReader(body, song.ID)
	if err != nil {
		log.Println("failed to create decrypting reader for song", err)
		return
	}
	if deezer.FLAC == quality {
		tagFLAC(dz.Client, reader, file, song)
	} else {
		tagMP3(dz.Client, file, song)
	}
	_, err = io.Copy(file, reader)
	if err != nil {
		log.Println("failed to download song", err)
		return
	}
	fmt.Println("downloaded", filepath)
}

func clean(name string) (cleaned string) {
	cleaned = strings.Replace(name, string(filepath.Separator), "", -1)
	cleaned = strings.Replace(cleaned, string(filepath.ListSeparator), "", -1)
	return
}

func songFilepath(song deezer.Song, quality deezer.Quality) string {
	return fmt.Sprintf("%s/%s/%d - %s.%s",
		clean(song.ArtistName),
		clean(song.AlbumTitle),
		song.TrackNumber,
		clean(song.Title),
		ext(quality),
	)
}

func artFilepath(song deezer.Song) string {
	return fmt.Sprintf("%s/%s/cover.jpg",
		clean(song.ArtistName),
		clean(song.AlbumTitle),
	)
}

func ext(q deezer.Quality) string {
	if q == deezer.FLAC {
		return "flac"
	}
	return "mp3"
}

func (dz *dzdown) downloadCover(song deezer.Song) error {
	f, err := os.OpenFile(artFilepath(song), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0666)
	if errors.Is(err, os.ErrExist) {
		return nil
	} else if err != nil {
		return err
	}
	defer f.Close()
	coverurl := fmt.Sprintf(
		"https://e-cdns-images.dzcdn.net/images/cover/%s/%dx%[2]d-000000-80-0-0.jpg",
		song.AlbumPicture, dz.artSize)
	resp, err := dz.Get(coverurl)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}
