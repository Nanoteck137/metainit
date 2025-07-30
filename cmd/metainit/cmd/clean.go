package cmd

import (
	"fmt"
	"log"
	"os"
	"path"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

type DirCleaning struct {
	Files []string
	Dirs  []string
}

func readDirForCleaning(dir string) (DirCleaning, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return DirCleaning{}, err
	}

	var files []string
	var dirs []string

	for _, entry := range entries {
		name := entry.Name()
		p := path.Join(dir, name)

		ext := path.Ext(name)

		if entry.IsDir() {
			dirs = append(dirs, p)
		}

		switch ext {
		case ".nfo", ".png", ".jpg", ".jpeg":
			files = append(files, p)
		}
	}

	return DirCleaning{
		Files: files,
		Dirs:  dirs,
	}, nil
}

var cleanCmd = &cobra.Command{
	Use: "clean",
	Run: func(cmd *cobra.Command, args []string) {
		dir, _ := cmd.Flags().GetString("dir")

		d, err := readDirForCleaning(dir)
		if err != nil {
			log.Fatal(err)
		}

		for _, dir := range d.Dirs {
			r, err := readDirForCleaning(dir)
			if err != nil {
				log.Fatal(err)
			}

			d.Files = append(d.Files, r.Files...)
		}

		if len(d.Files) == 0 {
			fmt.Println("Nothing to cleanup")
			return
		}

		fmt.Println("Files to be deleted:")
		for _, file := range d.Files {
			fmt.Printf("%s\n", file)
		}

		var confirmed bool
		form := huh.NewConfirm().
			Title("Delete files?").
			Value(&confirmed)
		err = form.Run()
		if err != nil {
			log.Fatal(err)
		}

		if confirmed {
			for _, file := range d.Files {
				err := os.Remove(file)
				if err != nil {
					log.Fatal(err)
				}
			}
		}
	},
}

func init() {
	cleanCmd.Flags().StringP("dir", "d", ".", "")

	rootCmd.AddCommand(cleanCmd)
}
