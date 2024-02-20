package main

import (
	"fmt"
	"math/rand"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

type model struct {
	heading string
	index   int
}

func newModel() model {
	return model{
		heading: "Hello world",
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func randomHeading() (string, int) {
	randomIndex := rand.Intn(3) // 0-2
	headings := []string{"first", "second", "third"}
	return headings[randomIndex], randomIndex
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "enter", " ":
			m.heading, m.index = randomHeading()
		}
	}
	return m, nil
}

func (m model) View() string {
	s := fmt.Sprintf("%v, %v", m.heading, m.index)
	return s
}

func main() {
	p := tea.NewProgram(newModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("error occured: %v", err)
		os.Exit(1)
	}
}
