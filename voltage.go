package main

import (
	"fmt"
	"math/rand"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

type texts struct {
	headings []string
}

func setPolish() texts {
	return texts{
		headings: []string{"pierwszy", "drugi", "trzeci"},
	}
}

func setEnglish() texts {
	return texts{
		headings: []string{"first", "second", "third"},
	}
}

type model struct {
	texts     texts
	heading   string
	index     int
	completed bool
	form      *huh.Form
}

func newModel() model {
	return model{
		heading: "Hello world",
		index:   0,
		form: huh.NewForm(huh.NewGroup(
			huh.NewSelect[string]().
				Key("lang").
				Options(huh.NewOptions[string]("EN", "PL")...).
				Title("Choose your language / Wybierz język..."),
		),
		),
	}
}

func (m model) Init() tea.Cmd {
	return m.form.Init()
}

func randomHeading() int {
	return rand.Intn(3) // 0-2
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}
	}
	if m.form.State != huh.StateNormal {
		if !m.completed {
			m.completed = true
			if m.form.Get("lang") == "EN" {
				m.texts = setEnglish()
			} else {
				m.texts = setPolish()
			}
		}
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "enter", " ":
				m.index = randomHeading()
				m.heading = m.texts.headings[m.index]
			}
		}
	} else {
		form, cmd := m.form.Update(msg)
		if f, ok := form.(*huh.Form); ok {
			m.form = f
		}
		return m, cmd
	}
	return m, nil
}

func (m model) View() string {
	if m.form.State == huh.StateNormal {
		return m.form.View()
	}
	s := fmt.Sprintf("%v\n%v, %v", m.form.Get("lang"), m.heading, m.index)
	return s
}

func main() {
	p := tea.NewProgram(newModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("error occured: %v", err)
		os.Exit(1)
	}
}
