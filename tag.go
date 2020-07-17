package main

import (
	"fmt"
	"io"
	"io/ioutil"

	"github.com/bogem/id3v2"
	"github.com/erebid/go-deezer/deezer"
)

func tagMP3(c *deezer.Client, w io.Writer, s deezer.Song) error {
	tag := id3v2.NewEmptyTag()
	tag.SetArtist(s.ArtistName)
	tag.SetAlbum(s.AlbumTitle)
	tag.SetTitle(s.Title)
	url := fmt.Sprintf("https://e-cdns-images.dzcdn.net/images/cover/%s/800x800-000000-80-0-0.jpg", s.AlbumPicture)
	resp, err := c.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	artwork, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	pic := id3v2.PictureFrame{
		Encoding:    id3v2.EncodingUTF8,
		MimeType:    "image/jpeg",
		PictureType: id3v2.PTFrontCover,
		Description: "Front cover",
		Picture:     artwork,
	}
	tag.AddAttachedPicture(pic)
	_, err = tag.WriteTo(w)
	return err
}
