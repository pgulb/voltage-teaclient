package main

import (
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/joho/godotenv"
)

type texts struct {
	headings []string
}
type rcFileCheck bool

// type rcFileCreated bool
type rcFileContent map[string]string
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
		panic(err)
	}
	info, err := os.Stat(rcPath)
	if err != nil {
		return rcFileCheck(false)
	}
	return rcFileCheck(!info.IsDir())
}

func createRcFile() tea.Msg {
	rcDir, err := rcDirName()
	if err != nil {
		panic(err)
	}
	rcFile, err := rcFileName()
	if err != nil {
		panic(err)
	}
	_, err = os.Stat(rcDir)
	if err != nil {
		err = os.Mkdir(rcDir, 0700)
		if err != nil {
			panic(err)
		}
	}
	resp, err := http.Get("https://raw.githubusercontent.com/pgulb/voltage/main/.voltagerc")
	if err != nil {
		panic(err)
	}
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	err = os.WriteFile(rcFile, content, 0600)
	if err != nil {
		panic(err)
	}
	cfg, err := godotenv.UnmarshalBytes(content)
	if err != nil {
		panic(err)
	}
	return rcFileContent(cfg)
}

func loadRc() tea.Msg {
	rc, err := rcFileName()
	if err != nil {
		panic(err)
	}
	cfg, err := godotenv.Read(rc)
	if err != nil {
		panic(err)
	}
	return rcFileContent(cfg)
}

func updateRc(cfg map[string]string) tea.Cmd {
	return func() tea.Msg {
		content, err := godotenv.Marshal(cfg)
		if err != nil {
			panic(err)
		}
		rc, err := rcFileName()
		if err != nil {
			panic(err)
		}
		err = os.WriteFile(rc, []byte(content), 0600)
		if err != nil {
			panic(err)
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

func randomHeading() int {
	return rand.Intn(3) // 0-2
}

func RerollHeading() tea.Msg {
	return newHeading(randomHeading())
}

type model struct {
	api                 string
	locale              string
	texts               texts
	heading             string
	index               int
	formLocaleProcessed bool
	formLocale          *huh.Form
	rcFileChecked       bool
}

func newModel() model {
	return model{
		formLocale: huh.NewForm(huh.NewGroup(
			huh.NewSelect[string]().
				Key("lang").
				Options(huh.NewOptions[string]("EN", "PL")...).
				Title("Choose your language / Wybierz język..."),
		),
		),
		rcFileChecked: false,
	}
}

func (m model) Init() tea.Cmd {
	return m.formLocale.Init()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		log.Println("tea.KeyMsg (initial)")
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}
	case rcFileCheck:
		log.Println("got rcFileCheck")
		m.rcFileChecked = true
		if !msg {
			return m, createRcFile
		} else {
			return m, loadRc
		}
	case rcFileContent:
		log.Println("got rcFileContent")
		m.api = msg["VOLTAGE_API_URL"]
		m.locale = msg["VOLTAGE_LOCALE"]
	}
	if !m.rcFileChecked {
		log.Println("requesting checkForRcFile")
		return m, checkForRcFile
	}
	if m.locale != "" {
		log.Println("disabling locale form")
		m.formLocale.State = huh.StateAborted
	}
	if m.formLocale.State != huh.StateNormal {
		if !m.formLocaleProcessed {
			m.formLocaleProcessed = true
			if m.locale == "" {
				log.Println("getting locale from filled form")
				m.locale = m.formLocale.GetString("lang")
				cfg := make(map[string]string)
				cfg["VOLTAGE_API_URL"] = m.api
				cfg["VOLTAGE_LOCALE"] = m.locale
				return m, updateRc(cfg)
			}
			log.Println("requesting heading for locale from rc file")
			if m.locale == "EN" {
				m.texts = setEnglish()
				return m, RerollHeading
			} else {
				m.texts = setPolish()
				return m, RerollHeading
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
			m.index = int(msg)
			m.heading = m.texts.headings[m.index]
			log.Println("heading change from cmd")
			log.Println(m.index)
			log.Println(m.heading)
		case rcFileUpdated:
			log.Println("got rcFileUpdated")
			if m.locale == "EN" {
				m.texts = setEnglish()
				return m, RerollHeading
			} else {
				m.texts = setPolish()
				return m, RerollHeading
			}
		case tea.KeyMsg:
			log.Println("got tea.KeyMsg (later)")
			switch msg.String() {
			case "enter", " ":
				return m, RerollHeading
			}
		}
	} else {
		form, cmd := m.formLocale.Update(msg)
		if f, ok := form.(*huh.Form); ok {
			m.formLocale = f
		}
		if m.formLocale.GetString("lang") != "" {
			// create a heading if form was completed
			return m, tea.Sequence(cmd, RerollHeading)
		}
		return m, cmd
	}
	return m, nil
}

func (m model) View() string {
	if m.formLocale.State == huh.StateNormal {
		return m.formLocale.View()
	}
	log.Printf("locale from View: %v\n", m.locale)
	log.Printf("heading from View: %v\n", m.heading)
	s := fmt.Sprintf("%v\n%v - %v", m.locale, m.heading, m.index)
	return s
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
