package main

import (
	"io"
	"strconv"

	"github.com/bogem/id3v2"
	"github.com/go-flac/flacvorbis"
	"github.com/go-flac/go-flac"
	"github.com/godeezer/lib/deezer"
)

// tagMP3 writes ID3 tags to a writer given a song.
func tagMP3(c *deezer.Client, w io.Writer, s deezer.Song) error {
	tag := id3v2.NewEmptyTag()
	tag.SetArtist(s.ArtistName)
	tag.SetAlbum(s.AlbumTitle)
	tag.SetTitle(s.Title)
	tag.AddTextFrame(tag.CommonID("Track number/Position in set"), tag.DefaultEncoding(), strconv.Itoa(s.TrackNumber))

	_, err := tag.WriteTo(w)
	return err
}

// tagFLAC, given a song, writes FLAC (vorbis) metadata blocks to a writer. The
// reader is needed so that the STREAMINFO metadata block can be read.
func tagFLAC(c *deezer.Client, r io.Reader, w io.Writer, s deezer.Song) error {
	f, err := flac.ParseMetadata(r)
	if err != nil {
		return err
	}

	// The first metadata block is always the STREAMINFO metadata block which is
	// required. Any other blocks Deezer gives us are unneeded.
	f.Meta = f.Meta[:1]

	tag := flacvorbis.New()
	tag.Add(flacvorbis.FIELD_TITLE, s.Title)
	tag.Add(flacvorbis.FIELD_ARTIST, s.ArtistName)
	tag.Add(flacvorbis.FIELD_ALBUM, s.AlbumTitle)
	tag.Add(flacvorbis.FIELD_TRACKNUMBER, strconv.Itoa(s.TrackNumber))

	tagmeta := tag.Marshal()
	f.Meta = append(f.Meta, &tagmeta)

	w.Write([]byte("fLaC"))
	for i, m := range f.Meta {
		last := i == len(f.Meta)-1
		_, err := w.Write(m.Marshal(last))
		if err != nil {
			return err
		}
	}
	return err
}
