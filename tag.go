package main

import (
	"fmt"
	"io"
	"io/ioutil"

	"github.com/bogem/id3v2"
	"github.com/go-flac/flacpicture"
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

	cover, err := cover(c, s.AlbumPicture)
	if err != nil {
		return err
	}
	pic := id3v2.PictureFrame{
		Encoding:    id3v2.EncodingUTF8,
		MimeType:    "image/jpeg",
		PictureType: id3v2.PTFrontCover,
		Description: "Front cover",
		Picture:     cover,
	}
	tag.AddAttachedPicture(pic)

	_, err = tag.WriteTo(w)
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
	tagmeta := tag.Marshal()

	cover, err := cover(c, s.AlbumPicture)
	if err != nil {
		return err
	}
	picture, err := flacpicture.NewFromImageData(
		flacpicture.PictureTypeFrontCover, "Front cover", cover, "image/jpeg")
	picturemeta := picture.Marshal()

	f.Meta = append(f.Meta, &tagmeta, &picturemeta)

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

func cover(c *deezer.Client, albpic string) ([]byte, error) {
	coverurl := fmt.Sprintf("https://e-cdns-images.dzcdn.net/images/cover/%s/800x800-000000-80-0-0.jpg", albpic)
	resp, err := c.Get(coverurl)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}
