package cmd

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/gosimple/slug"
	"github.com/nanoteck137/metainit/watchbook"
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

const XmlHeader = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n"

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

type Movie struct {
	XMLName xml.Name `xml:"movie"`
	Title   string   `xml:"title"`
}

type AnimeEntrySeasonType string

type AnimeEntrySeason struct {
	Path         string `toml:"path"`
	SeasonName   string `toml:"seasonName"`
	SeasonNumber int    `toml:"seasonNumber"`
	WatchbookId  string `toml:"watchbookId"`
}

type AnimeEntry struct {
	Title                 string             `toml:"serieTitle"`
	WatchbookCollectionId string             `toml:"watchbookCollectionId"`
	Seasons               []AnimeEntrySeason `toml:"seasons"`
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

	_, err = f.WriteString(XmlHeader)
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

func Slug(s string) string {
	return slug.Make(s)
}

type Collection struct {
	watchbook.GetCollectionById

	Items        [][]watchbook.CollectionItem
	SearchMapped map[string]watchbook.CollectionItem
}

func GetCollection(client *watchbook.Client, colId string) (*Collection, error) {
	collection, err := client.GetCollectionById(colId, watchbook.Options{})
	if err != nil {
		return nil, err
	}

	rawColItems, err := client.GetCollectionItems(colId, watchbook.Options{})
	if err != nil {
		return nil, err
	}

	items := make(map[string]watchbook.CollectionItem)

	for _, order := range rawColItems.Items {
		for _, item := range order {
			items[item.SearchSlug] = item
		}
	}

	return &Collection{
		GetCollectionById: *collection,
		Items:             rawColItems.Items,
		SearchMapped:      items,
	}, nil
}

func downloadImage(url, outDir, name string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("downloadImage: failed http get request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("downloadImage: download unsuccessfull: %s", resp.Status)
	}

	contentType := resp.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return "", fmt.Errorf("downloadImage: failed to parse Content-Type: %w", err)
	}

	ext := ""
	switch mediaType {
	case "image/png":
		ext = ".png"
	case "image/jpeg":
		ext = ".jpeg"
	default:
		return "", fmt.Errorf("downloadImage: unsupported media type: %s", mediaType)
	}

	out := path.Join(outDir, name+ext)

	f, err := os.OpenFile(out, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return "", fmt.Errorf("downloadImage: failed to open output file: %w", err)
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	if err != nil {
		return "", fmt.Errorf("downloadImage: failed io.Copy: %w", err)
	}

	return out, nil
}

type MetaType string

const (
	MetaTypeTV    MetaType = "tv"
	MetaTypeMovie MetaType = "movie"
)

type Meta struct {
	Type MetaType `json:"type"`
	Id   string   `json:"id"`
}

func readMeta(p string) (Meta, error) {
	d, err := os.ReadFile(p)
	if err != nil {
		return Meta{}, err
	}

	var meta Meta
	err = json.Unmarshal(d, &meta)
	if err != nil {
		return Meta{}, err
	}

	return meta, nil
}

func runCollectionQuery(client *watchbook.Client) (string, error) {
	var searchQuery string
	var colId string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Search Query").
				Value(&searchQuery),
			huh.NewSelect[string]().
				Title("Collections").
				OptionsFunc(func() []huh.Option[string] {
					cols, err := client.GetCollections(watchbook.Options{
						Query: map[string][]string{
							"filter": {fmt.Sprintf("name %% \"%%%s%%\"", searchQuery)},
						},
					})

					if err != nil {
						fmt.Println(err)
					}

					var options []huh.Option[string]
					if cols != nil {
						for _, c := range cols.Collections {
							options = append(options, huh.NewOption[string](c.Name, c.Id))
						}
					}

					return options
				}, &searchQuery).
				Value(&colId),
		),
	)

	err := form.Run()
	if err != nil {
		return "", err
	}

	return colId, nil
}

func runMediaQuery(client *watchbook.Client) (string, error) {
	var searchQuery string
	var mediaId string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Search Query").
				Value(&searchQuery),
			huh.NewSelect[string]().
				Title("Media").
				OptionsFunc(func() []huh.Option[string] {
					media, err := client.GetMedia(watchbook.Options{
						Query: map[string][]string{
							"filter": {fmt.Sprintf("title %% \"%%%s%%\"", searchQuery)},
						},
					})

					if err != nil {
						fmt.Println(err)
					}

					var options []huh.Option[string]
					if media != nil {
						for _, c := range media.Media {
							options = append(options, huh.NewOption[string](c.Title, c.Id))
						}
					}

					return options
				}, &searchQuery).
				Value(&mediaId),
		),
	)

	err := form.Run()
	if err != nil {
		return "", err
	}

	return mediaId, nil
}

func runTVMode(client *watchbook.Client, dir, colId string) error {
	collection, err := GetCollection(client, colId)
	if err != nil {
		return err
	}

	tv := TvShow{
		Title:     collection.Name,
		ShowTitle: collection.Name,
	}

	err = writeXml(path.Join(dir, "tvshow.nfo"), tv)
	if err != nil {
		return err
	}

	if collection.CoverUrl != nil {
		_, err := downloadImage(*collection.CoverUrl, dir, "poster")
		if err != nil {
			return err
		}
	}

	if collection.LogoUrl != nil {
		_, err := downloadImage(*collection.LogoUrl, dir, "logo")
		if err != nil {
			return err
		}
	}

	if collection.BannerUrl != nil {
		_, err := downloadImage(*collection.BannerUrl, dir, "backdrop")
		if err != nil {
			return err
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}

		p := path.Join(dir, name)

		if entry.IsDir() {
			s := Slug(name)
			found, ok := collection.SearchMapped[s]
			if !ok {
				fmt.Printf("No mapping for: %s\n", name)
				os.Exit(-1)
			}

			seasonNumber := 0

			if strings.Contains(s, "season-") {
				n := strings.TrimPrefix(s, "season-")
				seasonNumber, _ = strconv.Atoi(n)
			}

			if found.CoverUrl != nil {
				_, err := downloadImage(*found.CoverUrl, p, "poster")
				if err != nil {
					return err
				}
			}

			entries, err := os.ReadDir(p)
			if err != nil {
				return err
			}

			for _, entry := range entries {
				name := entry.Name()

				if strings.HasPrefix(name, ".") {
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
						return err
					}

					m := freg.FindStringSubmatch(name)

					if len(m) >= 3 {
						// TODO(patrik): Use the season.Number
						// season, _ := strconv.Atoi(m[1])
						episode, _ := strconv.Atoi(m[2])

						e := EpisodeDetails{
							Title:     fmt.Sprintf("Episode %d", episode),
							ShowTitle: collection.Name,
							Episode:   episode,
							Season:    seasonNumber,
						}

						err := writeXml(path.Join(p, n+".nfo"), e)
						if err != nil {
							return err
						}
					}
				}
			}
		}
	}

	meta := Meta{
		Type: MetaTypeTV,
		Id:   colId,
	}

	d, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}

	err = os.WriteFile(path.Join(dir, "meta.json"), d, 0644)
	if err != nil {
		return err
	}

	return nil
}

func isMediaExt(ext string) bool {
	switch ext {
	case ".mp4", ".mkv":
		return true
	}

	return false
}

func getAllMediaFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}

		ext := path.Ext(name)

		p := path.Join(dir, name)
		if isMediaExt(ext) {
			files = append(files, p)
		}
	}

	return files, nil
}

func runMovieMode(client *watchbook.Client, dir, mediaId string) error {
	media, err := client.GetMediaById(mediaId, watchbook.Options{})
	if err != nil {
		log.Fatal(err)
	}

	mediaFiles, err := getAllMediaFiles(dir)
	if err != nil {
		log.Fatal(err)
	}

	if len(mediaFiles) <= 0 {
		return fmt.Errorf("no media files found")
	}

	if len(mediaFiles) > 1 {
		return fmt.Errorf("more then one media files are not allowed for movies")
	}

	mediaFile := mediaFiles[0]
	mediaFileName := path.Base(mediaFile)
	mediaFileNameNoExt := strings.TrimSuffix(mediaFileName, path.Ext(mediaFileName))

	movie := Movie{
		Title: media.Title,
	}

	err = writeXml(path.Join(dir, mediaFileNameNoExt + ".nfo"), movie)
	if err != nil {
		return err
	}

	if media.CoverUrl != nil {
		_, err := downloadImage(*media.CoverUrl, dir, "poster")
		if err != nil {
			return err
		}
	}

	if media.LogoUrl != nil {
		_, err := downloadImage(*media.LogoUrl, dir, "logo")
		if err != nil {
			return err
		}
	}

	if media.BannerUrl != nil {
		_, err := downloadImage(*media.BannerUrl, dir, "backdrop")
		if err != nil {
			return err
		}
	}

	meta := Meta{
		Type: MetaTypeMovie,
		Id:   mediaId,
	}

	d, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}

	err = os.WriteFile(path.Join(dir, "meta.json"), d, 0644)
	if err != nil {
		return err
	}

	return nil
}

var runCmd = &cobra.Command{
	Use: "run",
	Run: func(cmd *cobra.Command, args []string) {
		serverAddress, _ := cmd.Flags().GetString("server-address")
		dir, _ := cmd.Flags().GetString("dir")

		client := watchbook.New(serverAddress)

		m, err := readMeta(path.Join(dir, "meta.json"))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				var sel string
				form := huh.NewSelect[string]().
					Title("Select type").
					Options(
						huh.NewOption[string]("TV", "tv"),
						huh.NewOption[string]("Movie", "movie"),
					).
					Value(&sel)

				err := form.Run()
				if err != nil {
					log.Fatal(err)
				}

				switch sel {
				case "tv":
					colId, err := runCollectionQuery(client)
					if err != nil {
						log.Fatal(err)
					}

					m = Meta{
						Type: MetaTypeTV,
						Id:   colId,
					}
				case "movie":
					mediaId, err := runMediaQuery(client)
					if err != nil {
						log.Fatal(err)
					}

					m = Meta{
						Type: MetaTypeMovie,
						Id:   mediaId,
					}
				}
			} else {
				log.Fatal(err)
			}
		}

		switch m.Type {
		case MetaTypeTV:
			err := runTVMode(client, dir, m.Id)
			if err != nil {
				log.Fatal(err)
			}
		case MetaTypeMovie:
			err := runMovieMode(client, dir, m.Id)
			if err != nil {
				log.Fatal(err)
			}
		}

	},
}

func init() {
	runCmd.Flags().StringP("server-address", "s", "https://watchbook.nanoteck137.net", "")
	runCmd.Flags().StringP("dir", "d", ".", "")

	rootCmd.AddCommand(runCmd)
}
