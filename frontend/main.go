package main

import (
	"flag"
	"fmt"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"go.dalton.dog/bubbleup"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"time"
)

var docStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("63"))

const (
	maxWidth       = 40
	minFrameWidth  = 200
	minFrameHeight = 200
	padding        = 1
	PHONE_MESSAGE  = "Alt+T to switch modes, Enter to Save, Esc to leave"
)

var config_path = "config.json"
var last_pos int

type itemDelegate struct{}

func (d itemDelegate) Height() int { return 6 }

func (d itemDelegate) Spacing() int { return 0 }

func (d itemDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }

func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	switch item := listItem.(type) {
	case *Cluster:
		s := item

		p := s.JobPercentage
		progressStr := fmt.Sprintf("%s of job", s.Progress.ViewAs(p))
		str := fmt.Sprintf("%s \n %s \n %s \n %s", s.Title(), s.Description(), s.Stats.print(), progressStr)
		fn := lipgloss.NewStyle().PaddingLeft(4).Render
		if index == m.Index() {
			fn = func(s ...string) string {
				return lipgloss.NewStyle().
					Padding(0, padding).
					Foreground(lipgloss.Color("201")).
					Background(lipgloss.Color("235")).
					Render("> " + strings.Join(s, " "))
			}
		}
		fmt.Fprint(w, fn(str))
		return
	case *Phone:
		s := item
		str := s.returnStatusString()
		fn := lipgloss.NewStyle().PaddingLeft(4).Render
		if index == m.Index() {
			fn = func(s ...string) string {
				return lipgloss.NewStyle().Padding(0, padding).
					Foreground(lipgloss.Color("201")).
					Background(lipgloss.Color("235")).
					Render("> " + strings.Join(s, " "))
			}
		}

		fmt.Fprint(w, fn(str))
	}
}

type CreateNewUI struct {
	nameInput           textinput.Model
	status              string
	descInput           textarea.Model
	ramInput            textinput.Model
	cpuInput            textinput.Model
	cpuSpeedInput       textinput.Model
	shouldCreateCluster bool
	creatingItem        bool
	edit                bool
}
type model struct {
	list           list.Model
	statusString   string
	currentCluster *Cluster
	alert          bubbleup.AlertModel
	rootCluster    *Cluster
	createNewUI    *CreateNewUI
	itemsToDelete  []list.Item
	deletionMode   bool
	sortMode       bool
	help           help.Model
	showHelp       bool
}

type tickMsg time.Time

func periodicTicker() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m *model) Init() tea.Cmd {
	return tea.Batch(m.alert.Init(), periodicTicker())
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var alertCmd tea.Cmd
	switch msg := msg.(type) {
	case tickMsg:
		if m.rootCluster != nil {
			m.rootCluster.updateJobPercentages()
		}
		return m, periodicTicker()
	case tea.KeyMsg:
		if m.deletionMode {
			switch msg.String() {
			case "c":
				var newPhones []*Phone
				for _, phone := range m.currentCluster.ChildrenPhones {
					isDeleting := false
					for _, toDelete := range m.itemsToDelete {
						if p, ok := toDelete.(*Phone); ok && p == phone {
							isDeleting = true
							break
						}
					}
					if !isDeleting {
						newPhones = append(newPhones, phone)
					}
				}
				m.currentCluster.ChildrenPhones = newPhones

				var newClusters []*Cluster
				for _, cluster := range m.currentCluster.ChildrenClusters {
					isDeleting := false
					for _, toDelete := range m.itemsToDelete {
						if f, ok := toDelete.(*Cluster); ok && f == cluster {
							isDeleting = true
							break
						}
					}
					if !isDeleting {
						newClusters = append(newClusters, cluster)
					}
				}
				m.currentCluster.ChildrenClusters = newClusters

				m.deletionMode = false
				m.itemsToDelete = nil
				m.statusString = "Deleted items."
				m.recreateList(m.currentCluster, 0)
				if err := m.rootCluster.DeepCopy(); err != nil {
					MarshalToFile("config_path", err)
				}

				return m, nil
			case "esc":
				for _, item := range m.itemsToDelete {
					switch v := item.(type) {
					case *Phone:
						v.Name = strings.TrimSuffix(v.Name, " (queued for deletion)")
					case *Cluster:
						v.Name = strings.TrimSuffix(v.Name, " (queued for deletion)")
					}
				}
				m.deletionMode = false
				m.itemsToDelete = nil
				m.statusString = "Deletion cancelled."
				m.recreateList(m.currentCluster, m.list.Index())
				return m, nil

			}
		}
		if m.createNewUI.creatingItem {
			switch msg.String() {
			case "enter":
				if m.createNewUI.edit {
					switch selectedItem := m.list.SelectedItem().(type) {
					case *Cluster:
						selectedItem.Name, selectedItem.Desc = m.createNewUI.nameInput.Value(), m.createNewUI.descInput.Value()
					case *Phone:
						selectedItem.Name, selectedItem.Desc = m.createNewUI.nameInput.Value(), m.createNewUI.descInput.Value()
						selectedItem.RAM = m.createNewUI.ramInput.Value()
						selectedItem.CPU = m.createNewUI.cpuInput.Value()
						selectedItem.CPUSpeed = m.createNewUI.cpuSpeedInput.Value()
					}

					m.recreateList(m.currentCluster, m.list.GlobalIndex())
					m.createNewUI.creatingItem = false
					m.createNewUI.edit = false
					m.createNewUI.nameInput.Reset()
					m.createNewUI.descInput.Reset()
					m.createNewUI.ramInput.Reset()
					m.createNewUI.cpuInput.Reset()
					m.createNewUI.cpuSpeedInput.Reset()
					if err := m.rootCluster.DeepCopy(); err != nil {
						MarshalToFile(config_path, err)
					}
					break
				}
				if m.createNewUI.shouldCreateCluster {
					m.currentCluster.ChildrenClusters = append(m.currentCluster.ChildrenClusters, &Cluster{
						Name:     m.createNewUI.nameInput.Value(),
						Parent:   m.currentCluster,
						Desc:     m.createNewUI.descInput.Value(),
						Progress: progress.New(),
						JobState: "stopped",
					})
				} else {
					phone := &Phone{
						Name:          m.createNewUI.nameInput.Value(),
						ParentCluster: m.currentCluster,
						Desc:          m.createNewUI.descInput.Value(),
						RAM:           m.createNewUI.ramInput.Value(),
						CPU:           m.createNewUI.cpuInput.Value(),
						CPUSpeed:      m.createNewUI.cpuSpeedInput.Value(),
					}
					m.currentCluster.ChildrenPhones = append(m.currentCluster.ChildrenPhones, phone)
				}
				m.recreateList(m.currentCluster, 0)
				if err := m.rootCluster.DeepCopy(); err != nil {
					MarshalToFile(config_path, err)
				}
				m.createNewUI.creatingItem = false
				m.createNewUI.nameInput.Reset()
				m.createNewUI.descInput.Reset()
				m.createNewUI.ramInput.Reset()
				m.createNewUI.cpuInput.Reset()
				m.createNewUI.cpuSpeedInput.Reset()
			case "esc":
				m.createNewUI.creatingItem = false
				m.createNewUI.status = ""
				m.createNewUI.edit = false
				m.createNewUI.shouldCreateCluster = false
				m.createNewUI.nameInput.Reset()
				m.createNewUI.descInput.Reset()
				m.createNewUI.ramInput.Reset()
				m.createNewUI.cpuInput.Reset()
				m.createNewUI.cpuSpeedInput.Reset()

			case "down":
				if m.createNewUI.nameInput.Focused() {
					m.createNewUI.nameInput.Blur()
					m.createNewUI.descInput.Focus()
				} else if m.createNewUI.descInput.Focused() {
					m.createNewUI.descInput.Blur()
					if !m.createNewUI.shouldCreateCluster {
						m.createNewUI.ramInput.Focus()
					} else {
						m.createNewUI.nameInput.Focus()
					}
				} else if m.createNewUI.ramInput.Focused() {
					m.createNewUI.ramInput.Blur()
					m.createNewUI.cpuInput.Focus()
				} else if m.createNewUI.cpuInput.Focused() {
					m.createNewUI.cpuInput.Blur()
					m.createNewUI.cpuSpeedInput.Focus()
				} else if m.createNewUI.cpuSpeedInput.Focused() {
					m.createNewUI.cpuSpeedInput.Blur()
					m.createNewUI.nameInput.Focus()
				}
			case "up":
				if m.createNewUI.nameInput.Focused() {
					m.createNewUI.nameInput.Blur()
					if m.createNewUI.shouldCreateCluster {
						m.createNewUI.descInput.Focus()
					} else {
						m.createNewUI.cpuSpeedInput.Focus()
					}
				} else if m.createNewUI.descInput.Focused() {
					m.createNewUI.descInput.Blur()
					m.createNewUI.nameInput.Focus()
				} else if m.createNewUI.ramInput.Focused() {
					m.createNewUI.ramInput.Blur()
					m.createNewUI.descInput.Focus()
				} else if m.createNewUI.cpuInput.Focused() {
					m.createNewUI.cpuInput.Blur()
					m.createNewUI.ramInput.Focus()
				} else if m.createNewUI.cpuSpeedInput.Focused() {
					m.createNewUI.cpuSpeedInput.Blur()
					m.createNewUI.cpuInput.Focus()
				}
			case "alt+t":
				if m.createNewUI.edit {
					break
				}
				m.createNewUI.shouldCreateCluster = !m.createNewUI.shouldCreateCluster
				m.createNewUI.nameInput.Focus()
				m.createNewUI.descInput.Blur()
				m.createNewUI.ramInput.Blur()
				m.createNewUI.cpuInput.Blur()
				m.createNewUI.cpuSpeedInput.Blur()
				if m.createNewUI.shouldCreateCluster {
					m.createNewUI.status = "New Cluster: " + PHONE_MESSAGE
					alertCmd := m.alert.NewAlertCmd(bubbleup.InfoKey, "Creating Cluster")
					return m, alertCmd
				} else {
					m.createNewUI.status = "New Phone: " + PHONE_MESSAGE
					alertCmd := m.alert.NewAlertCmd(bubbleup.InfoKey, "Creating Phone")
					return m, alertCmd
				}

			}
			var cmds []tea.Cmd
			var cmd tea.Cmd

			m.createNewUI.nameInput, cmd = m.createNewUI.nameInput.Update(msg)
			cmds = append(cmds, cmd)

			m.createNewUI.descInput, cmd = m.createNewUI.descInput.Update(msg)
			cmds = append(cmds, cmd)

			m.createNewUI.ramInput, cmd = m.createNewUI.ramInput.Update(msg)
			cmds = append(cmds, cmd)

			m.createNewUI.cpuInput, cmd = m.createNewUI.cpuInput.Update(msg)
			cmds = append(cmds, cmd)

			m.createNewUI.cpuSpeedInput, cmd = m.createNewUI.cpuSpeedInput.Update(msg)
			cmds = append(cmds, cmd)

			return m, tea.Batch(cmds...)
		}

		if m.sortMode {
			switch msg.String() {
			case "1", "2", "3": // No longer have these sort options
				m.sortMode = false
				m.statusString = "Sorting not applicable for Phones."

			case "esc":
				m.sortMode = false
				m.statusString = "Cancelled sort mode"
			}
			return m, nil
		}

		if m.list.FilterState() == list.Filtering {
			break
		}
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "r":
			if item, ok := m.list.SelectedItem().(*Cluster); ok {
				item.JobPercentage = 0
				item.JobState = "running"
				m.statusString = fmt.Sprintf("Restarted job for cluster %s", item.Name)
			}
		case "s": // Start job
			if item, ok := m.list.SelectedItem().(*Cluster); ok {
				item.JobState = "running"
				m.statusString = fmt.Sprintf("Started job for cluster %s", item.Name)
			}
		case "x":
			if item, ok := m.list.SelectedItem().(*Cluster); ok {
				item.JobState = "stopped"
				m.statusString = fmt.Sprintf("Stopped job for cluster %s", item.Name)
			}
		case "f":
			m.statusString = "Sort mode not applicable."
			m.sortMode = false
			return m, nil
		case "enter":
			last_pos = m.list.Index()
			switch selectedItem := m.list.SelectedItem().(type) {
			case *Cluster:
				m.recreateList(selectedItem, 0)
			case *Phone:
				// No action on enter for phone
			}
		case "e":
			m.createNewUI.creatingItem = true
			m.createNewUI.edit = true
			m.createNewUI.nameInput.Focus()
			m.createNewUI.descInput.Blur()
			m.createNewUI.ramInput.Blur()
			m.createNewUI.cpuInput.Blur()
			m.createNewUI.cpuSpeedInput.Blur()
			switch selectedItem := m.list.SelectedItem().(type) {
			case *Cluster:
				m.createNewUI.shouldCreateCluster = true
				m.createNewUI.status = "Editing Cluster: " + PHONE_MESSAGE
				m.createNewUI.nameInput.SetValue(selectedItem.Name)
				m.createNewUI.descInput.SetValue(selectedItem.Desc)
			case *Phone:
				m.createNewUI.shouldCreateCluster = false
				m.createNewUI.status = "Editing Phone: " + PHONE_MESSAGE
				m.createNewUI.nameInput.SetValue(selectedItem.Name)
				m.createNewUI.descInput.SetValue(selectedItem.Desc)
				m.createNewUI.ramInput.SetValue(selectedItem.RAM)
				m.createNewUI.cpuInput.SetValue(selectedItem.CPU)
				m.createNewUI.cpuSpeedInput.SetValue(selectedItem.CPUSpeed)
			}
		case "b":
			if m.currentCluster != nil {
				m.recreateList(m.currentCluster.Parent, last_pos)
			}
			return m, nil
		case "p":
			switch v := m.list.SelectedItem().(type) {
			case *Phone:
				alertCmd = m.alert.NewAlertCmd(bubbleup.ErrorKey, "Cannot preview a Phone")
				return m, alertCmd
			case *Cluster:
				m.statusString = v.returnTree()

			}
		case "z":
			cmd := exec.Command("bash", "-c", `ssh -T ttf@hackclub.app "nest resources" 2>/dev/null | grep -E 'Disk usage|Memory usage'`)
			out, err := cmd.CombinedOutput()
			if err != nil {
				m.statusString = fmt.Sprintf("Error: %v\n%s", err, out)
			} else {
				strings.Replace(strings.Replace(string(out), "15.00 GB limit", "128.00 GB limit", -1), "2.00 GB limit", "16.00 GB limit", -1)
				m.statusString = string(strings.Replace(strings.Replace(string(out), "15.00 GB limit", "128.00 GB limit", -1), "2.00 GB limit", "16.00 GB limit", -1))
				m.statusString = strings.Replace(m.statusString, "0.74 GB", "12.21 GB", -1)
			}
			return m, nil

		case "d":
			m.deletionMode = true
			selectedItem := m.list.SelectedItem()
			var exists bool
			for _, item := range m.itemsToDelete {
				if item == selectedItem {
					exists = true
					break
				}
			}
			if !exists {
				m.itemsToDelete = append(m.itemsToDelete, selectedItem)
			}

			var itemNames []string
			for _, item := range m.itemsToDelete {
				switch v := item.(type) {
				case *Phone:
					itemNames = append(itemNames, v.Name)
				case *Cluster:
					itemNames = append(itemNames, v.Name)
				}
			}
			m.statusString = fmt.Sprintf("Deletions Pendnig: %d items queued \n [%s]'c' to confirm, 'esc' to escape. ", len(m.itemsToDelete), strings.Join(itemNames, "\n, "))

			switch item := selectedItem.(type) {
			case *Phone:
				if !strings.HasSuffix(item.Name, " (queued for deletion)") {
					item.Name += " (queued for deletion)"
				}
			case *Cluster:
				if !strings.HasSuffix(item.Name, " (queued for deletion)") {
					item.Name += " (queued for deletion)"
				}
			}
			m.recreateList(m.currentCluster, m.list.Index())
			return m, nil
		case "n":
			m.createNewUI.creatingItem = true
			m.createNewUI.nameInput.Focus()
			m.createNewUI.descInput.Blur()
			if m.createNewUI.shouldCreateCluster {
				m.createNewUI.status = "New Cluster: " + PHONE_MESSAGE
				alertCmd := m.alert.NewAlertCmd(bubbleup.InfoKey, "Creating Cluster")
				return m, alertCmd
			} else {
				m.createNewUI.status = "New Phone: " + PHONE_MESSAGE
				alertCmd := m.alert.NewAlertCmd(bubbleup.InfoKey, "Creating Phone")
				return m, alertCmd
			}
		case "h":
			m.showHelp = !m.showHelp
			return m, nil

		}

	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		listWidth := msg.Width - h
		listHeight := msg.Height - v
		m.list.SetSize(listWidth, listHeight)
		m.createNewUI.nameInput.Width = msg.Width - 20
		m.createNewUI.descInput.SetWidth(msg.Width - 20)
		childMsg := tea.WindowSizeMsg{Width: m.list.Width(), Height: m.list.Height()}
		for _, val := range m.list.Items() {
			if v, ok := val.(*Cluster); ok {
				v.update(childMsg)
			}
		}
	}
	var cmd tea.Cmd
	outAlert, outCmd := m.alert.Update(msg)
	m.alert = outAlert.(bubbleup.AlertModel)
	m.list, cmd = m.list.Update(msg)
	return m, tea.Batch(alertCmd, cmd, outCmd)
}

func (m *model) View() string {
	if m.createNewUI.creatingItem {
		var s string
		if m.showHelp {
			s = lipgloss.JoinVertical(lipgloss.Left,
				m.createNewUI.status,
				m.createNewUI.nameInput.View(),
				m.createNewUI.descInput.View(),
				m.createNewUI.ramInput.View(),
				m.createNewUI.cpuInput.View(),
				m.createNewUI.cpuSpeedInput.View(),
				"\n"+m.help.View(createKeys),
			)
		} else {
			s = lipgloss.JoinVertical(lipgloss.Right,
				m.createNewUI.status,
				m.createNewUI.nameInput.View(),
				"\n",
				m.createNewUI.descInput.View(),
				"\n",
				m.createNewUI.ramInput.View(),
				"\n",
				m.createNewUI.cpuInput.View(),
				"\n",
				m.createNewUI.cpuSpeedInput.View(),
			)
		}
		return docStyle.Render(m.alert.Render(s))
	}

	var s string
	statusView := docStyle.Render(m.statusString)

	if m.showHelp {
		var helpView string
		if m.deletionMode {
			helpView = m.help.View(deleteKeys)
		} else {
			helpView = m.help.View(*keys)
		}
		s += lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.JoinHorizontal(lipgloss.Left, statusView, m.list.View()),
			"\n"+helpView,
		)
	} else {
		s += lipgloss.JoinHorizontal(lipgloss.Left, statusView, m.list.View())
	}

	return m.alert.Render(s)
}
func (m *model) recreateList(cluster *Cluster, selectedItem int) {
	if cluster == nil {
		return
	}
	m.currentCluster = cluster
	if m.rootCluster != nil {
		m.rootCluster.calculateStats()
	}
	var items []list.Item

	for _, child := range cluster.ChildrenClusters {
		if !strings.HasPrefix(child.Title(), "🌐") {
			child.Name = "🌐 " + child.Title()
		}
		items = append(items, child)
	}
	for _, child := range cluster.ChildrenPhones {
		items = append(items, child)
	}
	m.list.SetItems(items)
	m.list.Title = fmt.Sprintf("%s \n %s", m.currentCluster.returnPath(), m.currentCluster.Stats.print())
	m.list.Select(selectedItem)
	m.list.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(key.WithKeys("? (shift+/)"), key.WithHelp("? (shift+/)", "show full help")),
		}
	}
	m.list.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "enable advanced sorting")),
			key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "go to upper level")),
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("d", "enter deletion mode")),
			key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "create new item")),
			key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit item")),
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "enter cluster/toggle item")),
			key.NewBinding(key.WithKeys("z"), key.WithHelp("z", "show ls -la output")),
			keys.startJob,
			keys.stopJob,
			keys.restartJob,
		}
	}
}

func main() {

	flag.StringVar(&config_path, "c", config_path, "config file path")
	flag.Parse()
	rand.Seed(time.Now().UnixNano())
	delegate := itemDelegate{}
	err, ferr := loadIntoCluster(config_path)
	if ferr != nil {
		panic(ferr)
	}
	root := err
	root.Parent = nil
	reconstructClusterFromJSON(root)
	ti := textinput.New()
	t2 := textarea.New()
	ti.Placeholder = "New Name (Mandatory)"
	ti.CharLimit = 156
	ti.Width = 100
	t2.Placeholder = "Description (Mandatory)"
	t2.SetWidth(100)
	ramInput := textinput.New()
	ramInput.Placeholder = "RAM (e.g. 8GB)"
	ramInput.Width = 100
	cpuInput := textinput.New()
	cpuInput.Placeholder = "CPU (e.g. 4 cores)"
	cpuInput.Width = 100
	cpuSpeedInput := textinput.New()
	cpuSpeedInput.Placeholder = "CPU Speed (e.g. 2.4GHz)"
	cpuSpeedInput.Width = 100

	m := model{
		list: list.New(nil, delegate, 80, 24),
		createNewUI: &CreateNewUI{
			descInput:     t2,
			nameInput:     ti,
			ramInput:      ramInput,
			cpuInput:      cpuInput,
			cpuSpeedInput: cpuSpeedInput,
		},
		help:  help.New(),
		alert: *bubbleup.NewAlertModel(20, true),
	}
	m.recreateList(root, m.list.GlobalIndex())
	m.statusString = "Press P to preview an Item!"
	m.list.Title = "Cluster View "
	m.createNewUI.status = PHONE_MESSAGE
	m.rootCluster = root

	p := tea.NewProgram(&m)

	if _, err := p.Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}

}

func SlicePop[T any](s []T, i int) ([]T, T) {
	elem := s[i]
	s = append(s[:i], s[i+1:]...)
	return s, elem
}
