package main

import (
	"fmt"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"math/rand"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

type listKeyMap struct {
	previewItem  key.Binding
	reloadData   key.Binding
	goBack       key.Binding
	newPhone     key.Binding
	editItem     key.Binding
	deleteItem   key.Binding
	showHelp     key.Binding
	quit         key.Binding
	enterCluster key.Binding
	startJob     key.Binding
	stopJob      key.Binding
	restartJob   key.Binding
}
type itemKeyMap struct {
	goUp   key.Binding
	goDown key.Binding
	exit   key.Binding
	save   key.Binding
}

func newListKeyMap() *listKeyMap {
	return &listKeyMap{
		previewItem:  key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "preview cluster structure")),
		goBack:       key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "go to previous cluster")),
		reloadData:   key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "reload data")),
		newPhone:     key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new phone")),
		editItem:     key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit item")),
		deleteItem:   key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete item")),
		showHelp:     key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "toggle help")),
		quit:         key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		enterCluster: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "enter cluster/toggle phone")),
		startJob:     key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "start job")),
		stopJob:      key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "stop job")),
		restartJob:   key.NewBinding(key.WithKeys("ctrl+r"), key.WithHelp("ctrl+r", "restart job")),
	}
}

type Cluster struct {
	Name             string `json:"Name,omitempty"`
	Desc             string `json:"Desc,omitempty"`
	Progress         progress.Model
	Parent           *Cluster   `json:"Parent,omitempty"`
	ChildrenPhones   []*Phone   `json:"children_phones,omitempty"`
	ChildrenClusters []*Cluster `json:"children_clusters,omitempty"`
	Stats            Stats      `json:"Stats"`
	JobPercentage    float64    `json:"-"`
	JobState         string     `json:"-"` // "stopped", "running"
}

func (i *Cluster) Title() string       { return "🌐" + i.Name }
func (i *Cluster) Description() string { return i.Desc }
func (i *Cluster) FilterValue() string {
	return i.Name
}
func (i *Cluster) returnTree() string {
	s := "Cluster View \n"
	//TODO make this recursive?
	s += i.Desc + "\n"
	for _, item := range i.ChildrenClusters {
		s += item.Title() + "\n"
		for _, i := range item.ChildrenPhones {
			s += " - " + i.returnStatusString()

		}

	}
	for _, i := range i.ChildrenPhones {
		s += " -" + i.returnStatusString()
	}
	return s
}
func (i *Cluster) returnPath() string {
	var pathParts []string
	current := i
	pathParts = append(pathParts, "Root")
	for current != nil {
		pathParts = append(pathParts, current.Title())
		current = current.Parent
	}

	slices.Reverse(pathParts)
	return strings.Join(pathParts, " > ")
}
func (t *Phone) returnStatusString() string {
	var s string
	s += "📱 " + t.Title() + "\n"
	if t.Description() != "" {
		s += "\t" + t.Description() + "\n"
	}
	s += fmt.Sprintf("\tRAM: %s, CPU: %s, CPU Speed: %s", t.RAM, t.CPU, t.CPUSpeed)
	return s
}

func (i *Cluster) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		i.Progress.Width = msg.Width - padding*2 - 4
		if i.Progress.Width > maxWidth {
			i.Progress.Width = maxWidth
		}
		return nil
	default:
		return nil
	}
}

func (i *Cluster) View() string {

	pad := strings.Repeat(" ", padding)
	return "" + pad + i.Name + "" + pad + i.Progress.ViewAs(0.12) + "\n"
}

type Phone struct {
	ParentCluster *Cluster
	Name          string
	Desc          string
	RAM           string
	CPU           string
	CPUSpeed      string
}

func (k listKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.showHelp, k.quit}
}
func (k listKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.enterCluster, k.goBack, k.newPhone, k.editItem},              // first column
		{k.deleteItem, k.previewItem, k.reloadData, k.showHelp, k.quit}, // second column
		{k.startJob, k.stopJob, k.restartJob},                           // third column
	}
}

type createNewKeyMap struct {
	save       key.Binding
	cancel     key.Binding
	toggleType key.Binding
	nextField  key.Binding
	prevField  key.Binding
}

func newCreateNewKeyMap() createNewKeyMap {
	return createNewKeyMap{
		save:       key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "save")),
		cancel:     key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
		toggleType: key.NewBinding(key.WithKeys("alt+t"), key.WithHelp("alt+t", "toggle phone/cluster")),
		nextField:  key.NewBinding(key.WithKeys("down"), key.WithHelp("down", "next field")),
		prevField:  key.NewBinding(key.WithKeys("up"), key.WithHelp("up", "previous field")),
	}
}

func (k createNewKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.save, k.cancel}
}

func (k createNewKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.save, k.cancel, k.toggleType},
		{k.nextField, k.prevField},
	}
}

type deletionKeyMap struct {
	confirm key.Binding
	cancel  key.Binding
}

func newDeletionKeyMap() deletionKeyMap {
	return deletionKeyMap{
		confirm: key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "confirm deletion")),
		cancel:  key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel deletion")),
	}
}

func (k deletionKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.confirm, k.cancel}
}

func (k deletionKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.confirm, k.cancel},
	}
}

var keys = newListKeyMap()
var createKeys = newCreateNewKeyMap()
var deleteKeys = newDeletionKeyMap()

func (t *Phone) FilterValue() string { return t.Name }
func (t *Phone) Title() string       { return t.Name }
func (t *Phone) Description() string { return t.Desc }

type Stats struct {
	AvgRAM float64 `json:"AvgRAM,omitempty"`
	AvgCPU float64 `json:"AvgCPU,omitempty"`
}

func (s *Stats) print() string {
	var pp_string string
	pp_string += fmt.Sprintf("Average RAM: %.2f GB, ", s.AvgRAM)
	pp_string += fmt.Sprintf("Average CPU: %.2f Cores \n", s.AvgCPU)
	return pp_string
}

func (c *Cluster) calculateStats() (totalRAM float64, totalCPU float64, totalPhones int) {
	var currentRAM, currentCPU float64
	var currentPhones int
	re := regexp.MustCompile(`[0-9\.]+`)

	for _, phone := range c.ChildrenPhones {
		ramStr := phone.RAM
		ramNumStr := re.FindString(ramStr)
		ram, err := strconv.ParseFloat(ramNumStr, 64)
		if err == nil {
			currentRAM += ram
		}

		cpuStr := phone.CPU
		cpuNumStr := re.FindString(cpuStr)
		cpu, err := strconv.ParseFloat(cpuNumStr, 64)
		if err == nil {
			currentCPU += cpu
		}
		currentPhones++
	}

	for _, child := range c.ChildrenClusters {
		childRAM, childCPU, childPhones := child.calculateStats()
		currentRAM += childRAM
		currentCPU += childCPU
		currentPhones += childPhones
	}

	if currentPhones > 0 {
		c.Stats.AvgRAM = currentRAM / float64(currentPhones)
		c.Stats.AvgCPU = currentCPU / float64(currentPhones)
	} else {
		c.Stats.AvgRAM = 0
		c.Stats.AvgCPU = 0
	}

	return currentRAM, currentCPU, currentPhones
}

func (c *Cluster) updateJobPercentages() {
	if c.JobState == "running" {
		if c.JobPercentage < 1.0 {
			c.JobPercentage += rand.Float64() / 20.0 // Slower increment
		}
		if c.JobPercentage > 1.0 {
			c.JobPercentage = 1.0
		}
	}
	for _, child := range c.ChildrenClusters {
		child.updateJobPercentages()
	}
}
