package main

import (
	"bufio"
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/wfrank/qtools/pkg/qradar"
)

const (
	unixRefSetPrefix = "Managed UNIX Devices - "
)

func extractServerGroups(fileName string) (map[string][]string, error) {
	file, err := os.Open(fileName)
	if err != nil {
		return nil, fmt.Errorf("error opening csv file: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(bufio.NewReader(file))
	lines, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("error reading csv file: %v", err)
	}

	groups := make(map[string][]string)
	for i, line := range lines {
		if i == 0 {
			continue
		}
		var g string
		g = unixRefSetPrefix + line[2]
		groups[g] = append(groups[g], line[1])
		g = unixRefSetPrefix + strings.Join(line[2:], " - ")
		groups[g] = append(groups[g], line[1])
	}

	return groups, nil
}

func main() {

	url := flag.String("url", os.Getenv("QRADAR_BASE_URL"), "Base URL of the QRadar Console")
	file := flag.String("file", "USS-UNIX-Servers.csv", "UNIX Server List")
	token := flag.String("token", os.Getenv("QRADAR_SEC_TOKEN"), "Security Token of QRadar Authorized Service")
	flag.Parse()

	if *url == "" || *token == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}

	serverGroups, err := extractServerGroups(*file)
	if err != nil {
		log.Fatalf("error extracting server groups: %v", err)
		os.Exit(2)
	}

	qRadar := qradar.NewClient(*url, *token)

	allRefSets, err := qRadar.ReferenceSets()
	if err != nil {
		log.Fatalf("error retrieving reference sets: %v", err)
		os.Exit(3)
	}

	unixRefSets := make(map[string]*qradar.ReferenceSet)
	for _, set := range allRefSets {
		if name := set.Name; strings.HasPrefix(name, unixRefSetPrefix) {
			unixRefSets[name] = &set
		}
	}

	var wg sync.WaitGroup
	for group, servers := range serverGroups {
		wg.Add(1)
		go func(name string, servers []string) {
			defer wg.Done()
			_, ok := unixRefSets[name]
			if ok {
				log.Printf("reference set exists, purging: %q", name)
				task, err := qRadar.DeleteReferenceSet(name, true)
				if err != nil {
					log.Fatalf("error purging reference set: %v", err)
					return
				}
				for retry := 5; task.Status != "COMPLETED" && retry > 0; retry-- {
					task, err = qRadar.DeleteReferenceSetTaskStatus(task.ID)
					if err != nil {
						log.Fatalf("error polling reference set purging task status: %v", err)
						return
					}
					log.Printf("purging reference set: %q, status: %s", name, task.Status)
					if task.Status != "COMPLETED" {
						time.Sleep(1 * time.Second)
					}
				}
				if task.Status != "COMPLETED" {
					log.Fatalf("timeout purging reference set: %q, last status: %v", name, task.Status)
					return
				}
				log.Printf("purged reference set: %q", name)
			} else {
				log.Printf("creating reference set: %q", name)
				_, err := qRadar.CreateReferenceSet(name)
				if err != nil {
					log.Fatalf("error creating reference set: %v", err)
					return
				}
				log.Printf("created reference set: %q", name)
			}
			rs, err := qRadar.BulkLoadReferenceSet(name, servers)
			if err != nil {
				log.Fatalf("error bulkloading reference set: %q, %v", name, err)
				return
			}
			log.Printf("bulkloaded reference set: %q, number of elements: %d", name, rs.NumberOfElements)

		}(group, servers)
	}

	for name := range unixRefSets {
		if _, ok := serverGroups[name]; ok {
			continue
		}
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			log.Printf("reference set no longer needed, deleting: %q", name)
			task, err := qRadar.DeleteReferenceSet(name, false)
			if err != nil {
				log.Fatalf("error deleting reference set: %v", err)
				return
			}
			for retry := 5; task.Status != "COMPLETED" && retry > 0; retry-- {
				task, err = qRadar.DeleteReferenceSetTaskStatus(task.ID)
				if err != nil {
					log.Fatalf("error polling reference set deleting task status: %v", err)
					return
				}
				log.Printf("deleting reference set: %q, status: %s", name, task.Status)
				if task.Status != "COMPLETED" {
					time.Sleep(1 * time.Second)
				}
			}
			if task.Status != "COMPLETED" {
				log.Fatalf("timeout deleting reference set: %q, last status: %v", name, task.Status)
				return
			}
			log.Printf("deleted reference set: %q", name)
		}(name)
	}
	wg.Wait()
}
