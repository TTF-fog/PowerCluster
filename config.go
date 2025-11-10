package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/charmbracelet/bubbles/progress"
	"os"
)

func MarshalToFile(filename string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		panic(err)
	}

	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		return fmt.Errorf("error writing to file: %w", err)
	}

	return nil
}

func (f *Cluster) DeepCopy() *Cluster {
	if f == nil {
		return nil
	}

	newF := &Cluster{
		Name:  f.Name,
		Desc:  f.Desc,
		Stats: f.Stats,
	}

	if f.ChildrenPhones != nil {
		newF.ChildrenPhones = make([]*Phone, len(f.ChildrenPhones))
		for i, task := range f.ChildrenPhones {
			newF.ChildrenPhones[i] = task.deepCopy()
		}
	}

	if f.ChildrenClusters != nil {
		newF.ChildrenClusters = make([]*Cluster, len(f.ChildrenClusters))
		for i, childFolder := range f.ChildrenClusters {
			copiedChild := childFolder.DeepCopy()
			newF.ChildrenClusters[i] = copiedChild
		}
	}

	return newF
}

func (t *Phone) deepCopy() *Phone {
	if t == nil {
		return nil
	}

	newPhone := &Phone{
		Name:     t.Name,
		Desc:     t.Desc,
		RAM:      t.RAM,
		CPU:      t.CPU,
		CPUSpeed: t.CPUSpeed,
	}
	return newPhone
}

func loadIntoCluster(path string) (*Cluster, error) {
	f, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) == true {
		os.WriteFile(path, []byte("{}"), 0644)
	}
	var cluster Cluster
	_ = json.Unmarshal(f, &cluster)
	return &cluster, nil
}
func reconstructClusterFromJSON(cluster *Cluster) {
	cluster.JobState = "stopped"
	for _, item := range cluster.ChildrenClusters {
		item.Parent = cluster
		item.JobState = "stopped"
		reconstructClusterFromJSON(item)
	}
	for range cluster.ChildrenPhones {
		reconstructPhonesFromJSON(cluster)
	}
	if cluster.Parent != nil {
		cluster.Progress = progress.New()
		cluster.Progress.SetPercent(0)

	}

}
func reconstructPhonesFromJSON(cluster *Cluster) {
	for _, item := range cluster.ChildrenPhones {
		item.ParentCluster = cluster
	}
}
