package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/joho/godotenv"
)

type texts struct {
	headings []string
}

// game state enum
const (
	stateLoadingRc = iota
	stateRcLoadError
	stateFormLocale
	stateGame
)

type rcFileCheckResult struct {
	exist bool
	error error
}
type rcFileContent struct {
	content map[string]string
	error   error
}
type rcFileUpdated bool
type newHeading int

func rcFileName() (string, error) {
	rcPath, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	rcPath = filepath.Join(rcPath, "config", ".voltagerc")
	return rcPath, nil
}

func rcDirName() (string, error) {
	rcPath, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	rcPath = filepath.Join(rcPath, "config")
	return rcPath, nil
}

func checkForRcFile() tea.Msg {
	rcPath, err := rcFileName()
	if err != nil {
		return rcFileCheckResult{exist: false, error: err}
	}
	info, err := os.Stat(rcPath)
	if errors.Is(err, fs.ErrNotExist) {
		return rcFileCheckResult{exist: false, error: nil}
	}
	if err != nil {
		return rcFileCheckResult{exist: false, error: err}
	}
	return rcFileCheckResult{exist: !info.IsDir(), error: nil}
}

func createRcFile() tea.Msg {
	rcDir, err := rcDirName()
	if err != nil {
		return rcFileContent{
			content: nil,
			error:   err,
		}
	}
	rcFile, err := rcFileName()
	if err != nil {
		return rcFileContent{
			content: nil,
			error:   err,
		}
	}
	_, err = os.Stat(rcDir)
	if err != nil {
		err = os.Mkdir(rcDir, 0700)
		if err != nil {
			return rcFileContent{
				content: nil,
				error:   err,
			}
		}
	}
	resp, err := http.Get("https://raw.githubusercontent.com/pgulb/voltage/main/.voltagerc")
	if err != nil {
		return rcFileContent{
			content: nil,
			error:   err,
		}
	}
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return rcFileContent{
			content: nil,
			error:   err,
		}
	}
	err = os.WriteFile(rcFile, content, 0600)
	if err != nil {
		return rcFileContent{
			content: nil,
			error:   err,
		}
	}
	cfg, err := godotenv.UnmarshalBytes(content)
	if err != nil {
		return rcFileContent{
			content: nil,
			error:   err,
		}
	}
	return rcFileContent{
		content: cfg,
		error:   nil,
	}
}

func loadRc() tea.Msg {
	rc, err := rcFileName()
	if err != nil {
		return rcFileContent{
			content: nil,
			error:   err,
		}
	}
	cfg, err := godotenv.Read(rc)
	if err != nil {
		return rcFileContent{
			content: nil,
			error:   err,
		}
	}
	return rcFileContent{
		content: cfg,
		error:   nil,
	}
}

func updateRc(cfg map[string]string) tea.Cmd {
	return func() tea.Msg {
		content, err := godotenv.Marshal(cfg)
		if err != nil {
			return rcFileUpdated(false)
		}
		rc, err := rcFileName()
		if err != nil {
			return rcFileUpdated(false)
		}
		err = os.WriteFile(rc, []byte(content), 0600)
		if err != nil {
			return rcFileUpdated(false)
		}
		return rcFileUpdated(true)
	}
}

func setPolish() texts {
	return texts{
		headings: []string{
			"Nie wszystko złoto co się świeci.",
			"Lepiej późno niż wcale.",
			"Nie szata zdobi człowieka.",
		},
	}
}

func setEnglish() texts {
	return texts{
		headings: []string{
			"All that glitters is not gold.",
			"Better late than never.",
			"Clothes do not make the man.",
		},
	}
}

func randomHeading(currentIndex int) int {
	r := currentIndex
	for r == currentIndex {
		r = rand.Intn(3)
	}
	return r // 0-2
}

func RerollHeading(currentIndex int) tea.Cmd {
	return func() tea.Msg {
		return newHeading(randomHeading(currentIndex))
	}
}

type model struct {
	state               int
	error               error
	api                 string
	locale              string
	texts               texts
	heading             string
	headingIndex        int
	formLocaleProcessed bool
	formLocale          *huh.Form
}

func newModel() model {
	return model{
		state: stateLoadingRc,
		formLocale: huh.NewForm(huh.NewGroup(
			huh.NewSelect[string]().
				Key("lang").
				Options(huh.NewOptions[string]("EN", "PL")...).
				Title("Choose your language / Wybierz język..."),
		),
		),
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.formLocale.Init(),
		checkForRcFile,
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	// program exit combinations
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}
	}

	switch m.state {
	case stateLoadingRc:
		switch msg := msg.(type) {
		// creating basic .rc file or loading if detected existing
		case rcFileCheckResult:
			log.Println("got rcFileCheckResult")
			if msg.error != nil {
				log.Println(msg.error.Error())
				m.error = msg.error
				m.state = stateRcLoadError
				return m, nil
			}
			if !msg.exist {
				return m, createRcFile
			} else {
				return m, loadRc
			}

		// getting content of loaded .rc
		case rcFileContent:
			log.Println("got rcFileContent")

			if msg.error != nil {
				log.Println(msg.error.Error())
				m.error = msg.error
				m.state = stateRcLoadError
				return m, nil
			}
			m.api = msg.content["VOLTAGE_API_URL"]
			m.locale = msg.content["VOLTAGE_LOCALE"]
			m.state = stateFormLocale

			// if locale is set from .rc, no need for filling locale form
			// locale can be empty if .rc was downloaded from github
			if m.locale != "" {
				//if m.locale != "" && m.formLocale.State == huh.StateNormal {
				m.formLocale.State = huh.StateAborted
				m.state = stateGame
				return m, RerollHeading(m.headingIndex)
			}
		}

	case stateRcLoadError:
		time.Sleep(time.Second * 10)
		return m, tea.Quit

	case stateFormLocale:
		form, cmd := m.formLocale.Update(msg)
		if f, ok := form.(*huh.Form); ok {
			m.formLocale = f
		}
		if m.formLocale.GetString("lang") != "" {
			// roll a heading if form was completed
			m.state = stateGame
			return m, tea.Sequence(cmd, RerollHeading(m.headingIndex))
		}
		return m, cmd

	// if locale form is complete or aborted
	case stateGame:
		if !m.formLocaleProcessed {
			m.formLocaleProcessed = true

			// locale from form, write to .rc
			if m.locale == "" {
				log.Println("getting locale from filled form")
				m.locale = m.formLocale.GetString("lang")
				cfg := make(map[string]string)
				cfg["VOLTAGE_API_URL"] = m.api
				cfg["VOLTAGE_LOCALE"] = m.locale
				return m, updateRc(cfg)
			}

			log.Println("rerolling heading")
			if m.locale == "EN" {
				m.texts = setEnglish()
				return m, RerollHeading(m.headingIndex)
			} else {
				m.texts = setPolish()
				return m, RerollHeading(m.headingIndex)
			}
		}

		switch msg := msg.(type) {
		case newHeading:
			log.Println("got newHeading")
			if len(m.texts.headings) == 0 {
				log.Println("setting texts from newHeading")
				if m.locale == "EN" {
					m.texts = setEnglish()
				} else {
					m.texts = setPolish()
				}
			}
			m.headingIndex = int(msg)
			m.heading = m.texts.headings[m.headingIndex]

		// .rc updated
		case rcFileUpdated:
			if !msg {
				log.Println("error on rcFileUpdated")
				return m, nil
			}
			log.Println("got rcFileUpdated")
			if m.locale == "EN" {
				m.texts = setEnglish()
				return m, nil
			} else {
				m.texts = setPolish()
				return m, nil
			}

		case tea.KeyMsg:
			switch msg.String() {
			case " ":
				return m, RerollHeading(m.headingIndex)
			}
		}
	}

	return m, nil
}

func (m model) View() string {
	switch m.state {
	case stateLoadingRc:
		return "Loading...\n"
	case stateRcLoadError:
		return fmt.Sprint("error!\n ", m.error.Error())
	case stateFormLocale:
		return m.formLocale.View()
	case stateGame:
		s := fmt.Sprintf("%v\n%v - %v", m.locale, m.heading, m.headingIndex)
		return s
	default:
		return "unknown state: " + fmt.Sprint(m.state)
	}
}

func main() {
	f, err := os.OpenFile("game.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer f.Close()
	log.SetOutput(f)

	fmt.Print("\033[H\033[2J") // screen clear
	p := tea.NewProgram(newModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("error occured: %v", err)
		os.Exit(1)
	}
}
