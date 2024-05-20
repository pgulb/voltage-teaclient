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

type gameLines struct {
	headings []string
}

var lines gameLines

// game state enum
const (
	stateLoadingRc = iota
	stateRcLoadError
	stateFormLocale
	stateHeading
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

// rcDirName generates the directory name for configuration file.
//
// No parameters.
// Returns a string representing the directory path and an error if any.
func rcDirName() (string, error) {
	rcPath, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	rcPath = filepath.Join(rcPath, "config")
	return rcPath, nil
}

// rcFileName returns the path to the voltage configuration file.
//
// It has no parameters and returns a string and an error.
func rcFileName() (string, error) {
	rcDir, err := rcDirName()
	if err != nil {
		return "", err
	}
	rcPath := filepath.Join(rcDir, ".voltagerc")
	return rcPath, nil
}

// checkForRcFile checks for the existence of .voltagerc file.
//
// No parameters.
// Returns a tea.Msg.
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

// createRcFile creates .voltagerc file and returns its content or an error.
// Basic file is pulled from main branch of gh repo.
// No parameters.
// Returns a tea.Msg.
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

// loadRc loads the .voltagerc file and returns its content or an error message.
//
// No parameters.
// Returns a tea.Msg.
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

// updateRc updates the .voltagerc file with the provided configuration.
//
// The parameter is a map of string key-value pairs. The return type is a tea.Cmd.
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

// setLocale generates localisation and sets it inside gameLines struct.
//
// No parameters. No return values.
func setLocale(locale string) {
	if locale == "EN" {
		lines = gameLines{
			headings: []string{
				"All that glitters is not gold.",
				"Better late than never.",
				"Clothes do not make the man.",
			},
		}
	} else {
		lines = gameLines{
			headings: []string{
				"Nie wszystko złoto co się świeci.",
				"Lepiej późno niż wcale.",
				"Nie szata zdobi człowieka.",
			},
		}
	}
}

// randomHeading generates a random heading different from the currentIndex.
// Heading is visible after application finished setting locale.
//
// Takes an integer currentIndex as a parameter.
// Returns an integer.
func randomHeading(currentIndex int) int {
	r := currentIndex
	for r == currentIndex {
		r = rand.Intn(len(lines.headings))
	}
	return r
}

// RerollHeading generates a new heading different from current.
//
// currentIndex int
// tea.Cmd
func RerollHeading(currentIndex int, locale string) tea.Cmd {
	if len(lines.headings) == 0 {
		setLocale(locale)
	}
	return func() tea.Msg {
		return newHeading(randomHeading(currentIndex))
	}
}

type model struct {
	state               int
	error               error
	api                 string
	locale              string
	heading             string
	headingIndex        int
	formLocaleProcessed bool
	formLocale          *huh.Form
	formMainMenu        *huh.Form
}

// newModel initializes and returns a new model.
// Model represents state of application as a whole.
//
// No parameters.
// Returns a model.
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

// Init initializes the model.
// Runs a 'batch' of commands (unordered).
// Locale form is initialised and .voltagerc file check is scheduled.
//
// No parameters.
// Returns a tea.Cmd.
func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.formLocale.Init(),
		checkForRcFile,
	)
}

// Update updates the model based on the given message and returns
// the updated model and any command to execute.
// Update method is called for every incoming message.
// it reacts for user inputs like key presses and other messages
// like command outputs, file i/o, http requests etc.
//
// msg tea.Msg parameter, tea.Model and tea.Cmd return types.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	// program exit combinations
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}
	}

	// checking state of application
	switch m.state {
	// this state is used for loading .voltagerc file
	case stateLoadingRc:
		switch msg := msg.(type) {
		// creating basic .voltagerc file or loading if detected existing
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

		// getting content of loaded .voltagerc
		case rcFileContent:
			log.Println("got rcFileContent")
			log.Println(msg.content)

			if msg.error != nil {
				log.Println(msg.error.Error())
				m.error = msg.error
				m.state = stateRcLoadError
				return m, nil
			}
			m.api = msg.content["VOLTAGE_API_URL"]
			m.locale = msg.content["VOLTAGE_LOCALE"]
			m.state = stateFormLocale

			// if locale is set from .voltagerc, no need for filling locale form
			// locale can be empty if .voltagerc was downloaded from github
			if m.locale != "" {
				m.formLocale.State = huh.StateAborted
				m.state = stateHeading
				// return m, RerollHeading(m.headingIndex, m.locale)
				return m, RerollHeading(m.headingIndex, m.locale)
			}
		}

	// this state displays error message and exits
	case stateRcLoadError:
		time.Sleep(time.Second * 10)
		return m, tea.Quit

	// this state displays locale form
	case stateFormLocale:
		form, cmd := m.formLocale.Update(msg)
		if f, ok := form.(*huh.Form); ok {
			m.formLocale = f
		}
		if m.formLocale.GetString("lang") != "" {
			// roll a heading if form was completed
			m.state = stateHeading
			m.locale = m.formLocale.GetString("lang")
			return m, tea.Sequence(cmd, RerollHeading(m.headingIndex, m.locale))
		}
		return m, cmd

	// if locale form is complete or aborted
	case stateHeading:
		if !m.formLocaleProcessed {
			m.formLocaleProcessed = true

			// locale from form, write to .voltagerc
			// if m.locale == "" {
			if m.formLocale.GetString("lang") != "" {
				log.Println("getting locale from filled form")
				m.locale = m.formLocale.GetString("lang")
				cfg := make(map[string]string)
				cfg["VOLTAGE_API_URL"] = m.api
				cfg["VOLTAGE_LOCALE"] = m.locale
				return m, updateRc(cfg)
			}

			log.Println("rerolling heading")
			return m, RerollHeading(m.headingIndex, m.locale)
		}

		switch msg := msg.(type) {
		// rerolling heading
		case newHeading:
			log.Println("got newHeading")
			if len(lines.headings) == 0 {
				log.Println("setting gameLines from newHeading")
			}
			m.headingIndex = int(msg)
			m.heading = lines.headings[m.headingIndex]

		// .voltagerc updated
		case rcFileUpdated:
			if !msg {
				log.Println("error on rcFileUpdated")
				return m, nil
			}
			log.Println("got rcFileUpdated")
			return m, nil

		case tea.KeyMsg:
			switch msg.String() {
			case " ":
				return m, RerollHeading(m.headingIndex, m.locale)
			}
		}
	}

	return m, nil
}

// View returns the string representation of the model based on its state.
//
// No parameters.
// Returns a string.
func (m model) View() string {
	switch m.state {
	case stateLoadingRc:
		return "Loading...\n"
	case stateRcLoadError:
		return fmt.Sprint("error!\n ", m.error.Error())
	case stateFormLocale:
		return m.formLocale.View()
	case stateHeading:
		return m.heading
	default:
		return "unknown state: " + fmt.Sprint(m.state)
	}
}

// main is the entry point of the program.
//
// No parameters.
// No return values.
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
