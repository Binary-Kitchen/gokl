package main

import (
	"errors"
	"flag"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kr/pretty"
	git "gopkg.in/src-d/go-git.v4"
)

const datelayout = "2006-01-02"

type logentry struct {
	begin    time.Time
	end      time.Time
	topic    string
	appendix string
	media    []string
	body     string
}

func getRepo(repodir string, repourl string) error {
	var repo *git.Repository
	var err error
	repo, err = git.PlainClone(repodir, false, &git.CloneOptions{
		URL:      repourl,
		Progress: os.Stdout,
	})
	if err != nil {
		if err != git.ErrRepositoryAlreadyExists {
			return errors.New("Error while cloning repo:" + err.Error())
		}
		repo, err = git.PlainOpen(repodir)
		if err != nil {
			return errors.New("Error while opening repo:" + err.Error())
		}
	}
	tree, err := repo.Worktree()
	if err != nil {
		return errors.New("Error while creating worktree:" + err.Error())
	}
	err = tree.Pull(&git.PullOptions{
		RemoteName: "origin",
	})
	if err != nil {
		if err != git.NoErrAlreadyUpToDate {
			return errors.New("Error while pulling changes:" + err.Error())
		}
	}
	return nil
}

func parseFile(filepath string) (logentry, error) {
	var result logentry

	b, err := ioutil.ReadFile(filepath)
	if err != nil {
		return result, err
	}
	content := string(b)
	split := strings.Split(content, "\n\n")
	if len(split) != 2 {
		return result, errors.New("Invalid entry format")
	}
	header := split[0]
	body := split[1]

	split = strings.Split(header, "\n")
	for _, s := range split {
		if !strings.HasPrefix(s, "#") {
			split := strings.SplitN(s, ": ", 2)
			switch split[0] {
			case "BEGIN":
				t, err := time.Parse(datelayout, split[1])
				if err != nil {
					return result, errors.New("Error while parsing date:" + err.Error())
				}
				result.begin = t

			case "END":
				if split[1] != "None" {
					t, err := time.Parse(datelayout, split[1])
					if err != nil {
						return result, errors.New("Error while parsing date:" + err.Error())
					}
					result.end = t
				}
			case "TOPIC":
				result.topic = split[1]
			case "APPENDIX":
				result.appendix = split[1]
			case "MEDIA":
				split := strings.Split(split[1], ",")
				result.media = append(result.media, split[0])
			}
		}
	}
	result.body = body

	return result, nil
}

func generateLogEntries(repodir string) ([]logentry, error) {
	var result []logentry

	fis, err := ioutil.ReadDir(repodir)
	if err != nil {
		return result, err
	}

	for _, fi := range fis {
		if fi.Name() != "media" {
			if fi.IsDir() {
				yeardir := repodir + string(filepath.Separator) + fi.Name()
				fis2, err := ioutil.ReadDir(yeardir)
				if err != nil {
					return result, err
				}
				for _, fi2 := range fis2 {
					if fi2.IsDir() {
						monthdir := yeardir + string(filepath.Separator) + fi2.Name()
						fis3, err := ioutil.ReadDir(monthdir)
						if err != nil {
							return result, err
						}
						for _, fi3 := range fis3 {
							if !fi3.IsDir() {
								path := monthdir + string(filepath.Separator) + fi3.Name()
								le, err := parseFile(path)
								if err != nil {
									return nil, err
								}
								result = append(result, le)
							}
						}
					}
				}
			}
		}
	}

	return result, nil
}

func main() {
	var repodir string
	var gopherdir string
	var repourl string
	var imageurl string
	flag.StringVar(&repodir, "r", "./", "Directory for checking out the repository")
	flag.StringVar(&gopherdir, "g", "/var/gopher", "Directory for the generated gopher content")
	flag.StringVar(&repourl, "u", "https://github.com/Binary-Kitchen/kitchenlog.git", "URL of the log repository")
	flag.StringVar(&imageurl, "i", "https://raw.githubusercontent.com/Binary-Kitchen/kitchenlog/master/media/", "The URL for the raw image files")
	flag.Parse()

	err := getRepo(repodir, repourl)
	if err != nil {
		log.Fatal("Error getting Repo:", err)
	}
	les, err := generateLogEntries(repodir)
	if err != nil {
		log.Fatal("Error parsing Entries:", err)
	}
	pretty.Println(les)
}
