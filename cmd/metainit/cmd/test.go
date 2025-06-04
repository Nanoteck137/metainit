package cmd

import (
	"encoding/xml"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
)

// <?xml version="1.0" encoding="UTF-8" standalone="yes"?>
// <episodedetails>
//   <title>Episode 1</title>
//   <showtitle>Solo Leveling</showtitle>
//   <episode>1</episode>
//   <season>1</season>
//   <!-- <thumb aspect="poster">https://image.tmdb.org/t/p/original/geCRueV3ElhRTr0xtJuEWJt6dJ1.jpg</thumb> -->
//   <!-- <thumb aspect="clearlogo">https://image.tmdb.org/t/p/original/soogvWTvbW2YVpZwUz0SgPBKiVa.png</thumb> -->
//   <!-- <thumb aspect="poster" season="1" type="season">https://image.tmdb.org/t/p/original/dpCas1h6XQnmwvgGNNyf0USqyJC.jpg</thumb> -->
//   <!-- <thumb aspect="poster" season="2" type="season">https://image.tmdb.org/t/p/original/rsOApVLbIQEcNkqSlOxNPyg3FyI.jpg</thumb> -->
//   <!-- <fanart> -->
//   <!--   <thumb>https://image.tmdb.org/t/p/original/zN5hwgyGI5fQuJevzP4n7JynR5P.jpg</thumb> -->
//   <!-- </fanart> -->
// </episodedetails>

// <?xml version="1.0" encoding="UTF-8" standalone="yes"?>
// <tvshow>
//   <title>Solo Leveling</title>
//   <showtitle>Solo Leveling</showtitle>
//   <!-- <thumb aspect="poster">https://image.tmdb.org/t/p/original/geCRueV3ElhRTr0xtJuEWJt6dJ1.jpg</thumb> -->
//   <!-- <thumb aspect="clearlogo">https://image.tmdb.org/t/p/original/soogvWTvbW2YVpZwUz0SgPBKiVa.png</thumb> -->
//   <!-- <thumb aspect="poster" season="1" type="season">https://image.tmdb.org/t/p/original/dpCas1h6XQnmwvgGNNyf0USqyJC.jpg</thumb> -->
//   <!-- <thumb aspect="poster" season="2" type="season">https://image.tmdb.org/t/p/original/rsOApVLbIQEcNkqSlOxNPyg3FyI.jpg</thumb> -->
//   <!-- <fanart> -->
//   <!--   <thumb>https://image.tmdb.org/t/p/original/zN5hwgyGI5fQuJevzP4n7JynR5P.jpg</thumb> -->
//   <!-- </fanart> -->
// </tvshow>

var freg = regexp.MustCompile(`S(\d+)E(\d+)`)

const Header = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n"

type EpisodeDetails struct {
	XMLName   xml.Name `xml:"episodedetails"`
	Title     string   `xml:"title"`
	ShowTitle string   `xml:"showtitle"`
	Episode   int      `xml:"episode"`
	Season    int      `xml:"season"`
}

type TvShow struct {
	XMLName   xml.Name `xml:"tvshow"`
	Title     string   `xml:"title"`
	ShowTitle string   `xml:"showtitle"`
}

type AnimeEntrySeasonType string

type AnimeEntrySeason struct {
	DirName      string `toml:"dirName"`
	SeasonName   string `toml:"seasonName"`
	SeasonNumber int    `toml:"seasonNumber"`
	WatchbookId  string `toml:"watchbookId"`
}

type AnimeEntry struct {
	SerieTitle string             `toml:"serieTitle"`
	Seasons    []AnimeEntrySeason `toml:"seasons"`
}

type AnimeMovieEntry struct {
	MovieTitle  string `toml:"movieTitle"`
	WatchbookId string `toml:"watchbookId"`
}

func writeXml(p string, v any) error {
	d, err := xml.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}

	f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(Header)
	if err != nil {
		return err
	}

	_, err = f.Write(d)
	if err != nil {
		return err
	}

	return nil
}

func writeToml(p string, v any) error {
	d, err := toml.Marshal(v)
	if err != nil {
		return err
	}

	err = os.WriteFile(p, d, 0644)
	if err != nil {
		return err
	}

	return nil
}

var initCmd = &cobra.Command{
	Use: "init",
	Run: func(cmd *cobra.Command, args []string) {
		dir, _ := cmd.Flags().GetString("dir")
		output, _ := cmd.Flags().GetString("output")

		serieName := ""
		if a, err := filepath.Abs(dir); err == nil {
			serieName = path.Base(a)
		}

		e := AnimeEntry{
			SerieTitle: serieName,
			Seasons: []AnimeEntrySeason{},
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			log.Fatal(err)
		}

		for _, entry := range entries {
			name := entry.Name()
			if len(name) > 0 && name[0] == '.' {
				continue
			}

			if entry.IsDir() {
				e.Seasons = append(e.Seasons, AnimeEntrySeason{
					DirName:      name,
					SeasonName:   name,
					SeasonNumber: 0,
					WatchbookId:  "",
				})
			}
		}

		err = writeToml(path.Join(output, "serie.toml"), e)
		if err != nil {
			log.Fatal(err)
		}
	},
}

var genMetadataCmd = &cobra.Command{
	Use: "gen-metadata",
	Run: func(cmd *cobra.Command, args []string) {
		dir, _ := cmd.Flags().GetString("dir")

		d, err := os.ReadFile(path.Join(dir, "serie.toml"))
		if err != nil {
			log.Fatal(err)
		}

		var e AnimeEntry
		err = toml.Unmarshal(d, &e)
		if err != nil {
			log.Fatal(err)
		}

		tv := TvShow{
			Title:     e.SerieTitle,
			ShowTitle: e.SerieTitle,
		}

		err = writeXml(path.Join(dir, "tvshow.nfo"), tv)
		if err != nil {
			log.Fatal(err)
		}

		for _, season := range e.Seasons {
			p := path.Join(dir, season.DirName)
			entries, err := os.ReadDir(p)
			if err != nil {
				log.Fatal(err)
			}

			for _, entry := range entries {
				name := entry.Name()

				if len(name) > 0 && name[0] == '.' {
					continue
				}

				ext := path.Ext(name)

				switch ext {
				case ".mkv", ".mp4":
					n := strings.TrimSuffix(name, ext)

					cmd := exec.Command(
						"ffmpeg",
						"-i", path.Join(p, name),
						"-vf", "select='gte(t,9)',scale=720:-1",
						"-frames:v", "1",
						path.Join(p, fmt.Sprintf("%s-thumb.png", n)),
					)

					err = cmd.Run()
					if err != nil {
						log.Fatal(err)
					}

					m := freg.FindStringSubmatch(name)

					if len(m) >= 3 {
						// TODO(patrik): Use the season.Number
						season, _ := strconv.Atoi(m[1])
						episode, _ := strconv.Atoi(m[2])

						e := EpisodeDetails{
							Title:     fmt.Sprintf("Episode %d", episode),
							ShowTitle: e.SerieTitle,
							Episode:   episode,
							Season:    season,
						}

						err := writeXml(path.Join(p, n+".nfo"), e)
						if err != nil {
							log.Fatal(err)
						}
					}
				}
			}
		}
	},
}

func init() {
	initCmd.Flags().StringP("dir", "d", ".", "")
	initCmd.Flags().StringP("output", "o", ".", "Output Directory")

	genMetadataCmd.Flags().StringP("dir", "d", ".", "")

	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(genMetadataCmd)
}
