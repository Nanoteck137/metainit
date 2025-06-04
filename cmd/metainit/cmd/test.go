package cmd

import (
	"encoding/xml"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"

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

var freg = regexp.MustCompile(`S(\d+)E(\d+)`)

const Header = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n"

type EpisodeDetails struct {
	XMLName   xml.Name `xml:"episodedetails"`
	Title     string   `xml:"title"`
	ShowTitle string   `xml:"showtitle"`
	Episode   int      `xml:"episode"`
	Season    int      `xml:"season"`
}

type AnimeEntrySeasonType string

type AnimeEntrySeason struct {
	DirName     string `json:"dirName"`
	SeasonName  string `json:"seasonName"`
	WatchbookId string `json:"watchbookId"`
}

type AnimeEntry struct {
	SerieTitle string             `json:"serieTitle"`
	Seasons    []AnimeEntrySeason `json:"seasons"`
}

type AnimeMovieEntry struct {
	MovieTitle string `json:"movieTitle"`
	WatchbookId string `json:"watchbookId"`
}

var testCmd = &cobra.Command{
	Use: "test",
	Run: func(cmd *cobra.Command, args []string) {
		// e := AnimeEntry{
		// 	SerieTitle: "Solo Leveling",
		// 	Seasons: []AnimeEntrySeasons{
		// 		{
		// 			DirName:     "Season 1",
		// 			SeasonName:  "Season 1",
		// 			WatchbookId: "",
		// 		},
		// 		{
		// 			DirName:     "Season 2",
		// 			SeasonName:  "Season 2",
		// 			WatchbookId: "",
		// 		},
		// 	},
		// }
		// dir := "/Volumes/media2/anime/Solo Leveling"

		e := AnimeEntry{
			SerieTitle: "Jujutsu Kaisen",
			Seasons: []AnimeEntrySeason{
				{
					DirName:     "Season 01",
					SeasonName:  "Season 1",
					WatchbookId: "",
				},
				{
					DirName:     "Season 02",
					SeasonName:  "Season 2",
					WatchbookId: "",
				},
			},
		}
		dir := "/Volumes/media2/anime/Jujutsu Kaisen"

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
					// ffmpeg 

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

						d, err := xml.MarshalIndent(e, "", "  ")
						if err != nil {
							log.Fatal(err)
						}

						_ = d

						fmt.Printf("n: %v\n", n)

						{
							f, err := os.OpenFile(path.Join(p, n+".nfo"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
							if err != nil {
								log.Fatal(err)
							}
							defer f.Close()

							_, err = f.WriteString(Header)
							if err != nil {
								log.Fatal(err)
							}

							_, err = f.Write(d)
							if err != nil {
								log.Fatal(err)
							}
						}
					} else {
						e := EpisodeDetails{
							Title:     season.SeasonName,
							ShowTitle: e.SerieTitle,
							Episode:   0,
							Season:    0,
						}

						d, err := xml.MarshalIndent(e, "", "  ")
						if err != nil {
							log.Fatal(err)
						}

						_ = d

						n := strings.TrimSuffix(name, ext)
						fmt.Printf("n: %v\n", n)

						{
							f, err := os.OpenFile(path.Join(p, n+".nfo"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
							if err != nil {
								log.Fatal(err)
							}
							defer f.Close()

							_, err = f.WriteString(Header)
							if err != nil {
								log.Fatal(err)
							}

							_, err = f.Write(d)
							if err != nil {
								log.Fatal(err)
							}
						}
					}
				}

			}
		}
	},
}

func init() {
	rootCmd.AddCommand(testCmd)
}
