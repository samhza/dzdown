# Dzdown
A Deezer music downloader.

## DISCLAIMER
This tool is intended for educational and private use only, and not as a tool for pirating and distributing music!

Remember that the artists and studios put a lot of work into making music - purchase music to support them.

## Usage:

```
go get github.com/samhza/dzdown
dzdown -arl your_arl_token_here <links>
```
You can any number of links to songs, albums, or artists.

As an example, to download Kanye West's discography, run:
```
dzdown -arl your_arl_token_here https://www.deezer.com/us/artist/230
```
### Options
#### Song quality

You can specify what quality you want to download your music in using -q

To download the music as a flac you can run:
```
dzdown -arl your_arl_token_here -q flac https://www.deezer.com/us/artist/230
```
Valid options for the -q flag are mp3-128, mp3-320, and flac.

#### Concurrent downloads

Dzdown can download multiple songs concurrently, and by default limits itself
to only downloading up to 4 songs at a time. That limit can be overriden with the
-maxdl flag.

To download up to 8 songs at a time, you can use:
```
dzdown -arl your_arl_token_here -maxdl 8 https://www.deezer.com/us/album/117098022
```
