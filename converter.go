package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"
)

var instancePricing = map[string]float64{
	"g6-nanode-1":      0.0075,
	"g6-standard-1":    0.015,
	"g6-standard-2":    0.03,
	"g6-standard-4":    0.06,
	"g6-standard-6":    0.12,
	"g6-standard-8":    0.24,
	"g6-standard-16":   0.48,
	"g6-standard-20":   0.72,
	"g6-standard-24":   0.96,
	"g6-standard-32":   1.44,
	"g7-highmem-1":     0.09,
	"g7-highmem-2":     0.18,
	"g7-highmem-4":     0.36,
	"g7-highmem-8":     0.72,
	"g7-highmem-16":    1.44,
	"g6-dedicated-2":   0.045,
	"g6-dedicated-4":   0.09,
	"g6-dedicated-8":   0.18,
	"g6-dedicated-16":  0.36,
	"g6-dedicated-32":  0.72,
	"g6-dedicated-48":  1.08,
	"g6-dedicated-50":  1.44,
	"g6-dedicated-56":  2.88,
	"g6-dedicated-64":  5.76,
	"g1-gpu-rtx6000-1": 1.5,
	"g1-gpu-rtx6000-2": 3.0,
	"g1-gpu-rtx6000-3": 4.5,
	"g1-gpu-rtx6000-4": 6.0,
}

type FinalData struct {
	Label    string   `json:"label"`
	Created  string   `json:"created"`
	Type     string   `json:"type"`
	IPv4     []string `json:"ipv4"`
	Region   string   `json:"region"`
	Tag      string   `json:"tags"`
	FullTags string   `json:"fullTags"`
	// PriceSinceCreation
	PriceSinceCreation float64 `json:"priceSinceCreation"`
	PriceThisMonth     float64 `json:"priceThisMonth"`
	//Specs
	Disk     int `json:"disk"`
	Memory   int `json:"memory"`
	Vcpus    int `json:"vcpus"`
	Gpus     int `json:"gpus"`
	Transfer int `json:"transfer"`
}

type Data struct {
	ID              int      `json:"id"`
	Label           string   `json:"label"`
	Group           string   `json:"group"`
	Status          string   `json:"status"`
	Created         string   `json:"created"`
	Updated         string   `json:"updated"`
	Type            string   `json:"type"`
	IPv4            []string `json:"ipv4"`
	IPv6            string   `json:"ipv6"`
	Image           string   `json:"image"`
	Region          string   `json:"region"`
	Specs           Specs    `json:"specs"`
	Alerts          Alerts   `json:"alerts"`
	Backups         Backups  `json:"backups"`
	Hypervisor      string   `json:"hypervisor"`
	WatchdogEnabled bool     `json:"watchdog_enabled"`
	Tags            []string `json:"tags"`
	HostUUID        string   `json:"host_uuid"`
}

func (d Data) ToFinalData() FinalData {
	price := d.getPrice()
	priceThisMonth := d.getPriceSinceMonthStarted()
	if priceThisMonth > price {
		priceThisMonth = price
	}
	return FinalData{
		Label:              d.Label,
		Created:            d.Created,
		Type:               d.Type,
		IPv4:               d.IPv4,
		Region:             d.Region,
		FullTags:           strings.Join(d.Tags, ","),
		PriceSinceCreation: price,
		PriceThisMonth:     priceThisMonth,
		Disk:               d.Specs.Disk,
		Vcpus:              d.Specs.Vcpus,
		Gpus:               d.Specs.Gpus,
		Transfer:           d.Specs.Transfer,
		Memory:             d.Specs.Memory,
	}
}
func (d Data) getPriceSinceMonthStarted() float64 {
	instancePrice, ok := instancePricing[d.Type]
	if !ok {
		return -1
	}
	val, err := priceSince(beginningOfMonth, instancePrice)
	if err != nil {
		return -1
	}
	return val
}

func (d Data) getPrice() float64 {
	instancePrice, ok := instancePricing[d.Type]
	if !ok {
		return -1
	}
	val, err := priceSince(d.Created, instancePrice)
	if err != nil {
		return -1
	}
	return val
}

type Specs struct {
	Disk     int `json:"disk"`
	Memory   int `json:"memory"`
	Vcpus    int `json:"vcpus"`
	Gpus     int `json:"gpus"`
	Transfer int `json:"transfer"`
}

type Alerts struct {
	CPU           int `json:"cpu"`
	NetworkIn     int `json:"network_in"`
	NetworkOut    int `json:"network_out"`
	TransferQuota int `json:"transfer_quota"`
	IO            int `json:"io"`
}

type Schedule struct {
	Day    *string `json:"day"`
	Window *string `json:"window"`
}

type Backups struct {
	Enabled        bool     `json:"enabled"`
	Available      bool     `json:"available"`
	Schedule       Schedule `json:"schedule"`
	LastSuccessful *string  `json:"last_successful"`
}

type Response struct {
	Data []Data `json:"data"`
}

func main() {
	fileName := flag.String("filename", "vms.json", "CSV file to read")
	tags := flag.String("tags", "core,gnosis,chiado,gnosis-withdrawal-1,gnosis-withdrawal-2", "Comma-separated list of tags to filter by")
	outputFileName := flag.String("output", "vms.csv", "CSV file to write")
	flag.Parse()
	var searchTags = map[string][]FinalData{}
	for _, tag := range strings.Split(*tags, ",") {
		if tag != "" {
			searchTags[tag] = make([]FinalData, 0)
		}
	}

	jsonFile, err := os.Open(*fileName)
	if err != nil {
		fmt.Printf("Error opening file: %v", err)
		return
	}
	defer jsonFile.Close()

	jsonBytes, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		fmt.Printf("Error reading file: %v", err)
		return
	}

	var resp Response
	if err := json.Unmarshal(jsonBytes, &resp); err != nil {
		fmt.Printf("Error unmarshalling JSON: %v", err)
		return
	}

	finalData := make([]FinalData, 0)
	for _, entry := range resp.Data {
		for key := range searchTags {
			if containsTag(entry.Tags, key) {
				entryAsFinalData := entry.ToFinalData()
				entryAsFinalData.Tag = key
				finalData = append(finalData, entryAsFinalData)
				break
			}
		}
	}
	err = saveToCSV(finalData, *outputFileName)
	if err != nil {
		return
	}
}
func saveToCSV(data []FinalData, output string) error {
	file, err := os.Create(output)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	header := []string{"Label", "Tag", "FullTags", "PriceSinceCreation(hours)", "PriceThisMonth", "Created", "Type", "IPv4", "Region",
		"Disk", "Memory", "Vcpus", "Gpus", "Transfer"}
	writer.Write(header)

	for _, d := range data {
		var ipv4 string
		if len(d.IPv4) > 0 {
			ipv4 = strings.Join(d.IPv4, ",")
		} else {
			ipv4 = ""
		}

		line := []string{d.Label, d.Tag, d.FullTags, strconv.FormatFloat(d.PriceSinceCreation, 'f', 2, 64), strconv.FormatFloat(d.PriceThisMonth, 'f',
			2, 64), d.Created, d.Type, ipv4, d.Region,
			strconv.Itoa(d.Disk), strconv.Itoa(d.Memory), strconv.Itoa(d.Vcpus), strconv.Itoa(d.Gpus), strconv.Itoa(d.Transfer),
		}
		writer.Write(line)
	}

	return nil
}

func containsTag(tags []string, target string) bool {
	for _, tag := range tags {
		if tag == target {
			return true
		}
	}
	return false
}

func priceSince(dateStr string, price float64) (float64, error) {
	layout := "2006-01-02T15:04:05"
	date, err := time.Parse(layout, dateStr)
	if err != nil {
		return 0, err
	}

	now := time.Now()
	duration := now.Sub(date)
	days := duration.Hours()

	return days * price, nil
}

var beginningOfMonth = func() string {
	now := time.Now()
	year, month, _ := now.Date()
	return time.Date(year, month, 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02T15:04:05")
}()

func maxDate(a, b string) string {
	// parse strings to time
	t1, err := time.Parse(time.RFC3339, a)
	if err != nil {
		return b
	}
	t2, err := time.Parse(time.RFC3339, b)
	if err != nil {
		return a
	}
	// compare
	if t1.After(t2) {
		return a
	}
	return b
}
