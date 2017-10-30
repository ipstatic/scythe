package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"time"

	"gopkg.in/Iwark/spreadsheet.v2"
	"gopkg.in/yaml.v2"
)

var (
	configFile = flag.String(
		"config.file", "scythe.yml",
		"configuration file",
	)

	config *Config
)

// Config represents the YAML structure of the configuration file.
type Config struct {
	SpreadsheetID     string `yaml:"spreadsheet_id"`
	Employee          string `yaml:"employee"`
	Category          string `yaml:"category"`
	HarvestSubdomain  string `yaml:"harvest_subdomain"`
	HarvestUsername   string `yaml:"harvest_username"`
	HarvestUsernameID string `yaml:"harvest_username_id"`
	HarvestPassword   string `yaml:"harvest_password"`
}

type HarvestReport []struct {
	DayEntry struct {
		Notes     string  `json:"notes"`
		Hours     float64 `json:"hours"`
		ProjectID int     `json:"project_id"`
		TaskID    int     `json:"task_id"`
	} `json:"day_entry"`
}

type Week struct {
	Start              time.Time
	End                time.Time
	Of                 string
	Pto                float64
	BillableEntries    HarvestReport
	BillableHours      float64
	NonBillableEntries HarvestReport
	NonBillableHours   float64
}

func calculateHours(w *Week) {
	var billable float64 = 0
	for _, r := range w.BillableEntries {
		billable += r.DayEntry.Hours
	}
	w.BillableHours = billable

	var non float64 = 0
	for _, r := range w.NonBillableEntries {
		non += r.DayEntry.Hours
	}
	w.NonBillableHours = non
}

func findStartDate(t time.Time) time.Time {
	for t.Weekday() != time.Monday {
		t = t.AddDate(0, 0, -1)
	}

	return t
}

// findEndDate will return the date of the next Sunday if the passed date
// is not already a Sunday.
func findEndDate(t time.Time) time.Time {
	for t.Weekday() != time.Sunday {
		t = t.AddDate(0, 0, +1)
	}

	return t
}

// getTimeEntries will query Harvest for time entries for the given date range
func getTimeEntries(start time.Time, end time.Time, billable bool) HarvestReport {
	client := &http.Client{}

	var bill string
	if billable {
		bill = "yes"
	} else {
		bill = "no"
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("https://%s.harvestapp.com/people/%s/entries?from=%s&to=%s&billable=%s", config.HarvestSubdomain, config.HarvestUsernameID, start.Format("20060102"), end.Format("20060102"), bill), nil)
	req.SetBasicAuth(config.HarvestUsername, config.HarvestPassword)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}

	defer resp.Body.Close()

	var hr HarvestReport
	if err := json.NewDecoder(resp.Body).Decode(&hr); err != nil {
		log.Fatal(err)
	}

	return hr
}

func loadConfiguration(file string) (*Config, error) {
	yamlFile, err := ioutil.ReadFile(*configFile)
	if err != nil {
		return nil, err
	}

	config := new(Config)
	err = yaml.Unmarshal(yamlFile, &config)
	if err != nil {
		return nil, err
	}

	return config, nil
}

func main() {
	flag.Parse()

	var input_start string
	var input_end string
	var err error

	now := time.Now()
	start_date := findStartDate(now)
	end_date := findEndDate(now)

	config, err = loadConfiguration(*configFile)
	if err != nil {
		log.Fatalf("Configuration error, aborting: %s", err)
	}

	// First we prompt for a start date. This will be the start of the reporting
	// period. We will default to the Monday of the current week. If only 2 digits
	// are entered, we assume they mean this year and month otherwise they will
	// need to enter the full YYYY-MM-DD date in order to set the start date.
	fmt.Printf("Start Date (%s): ", start_date.Format("2006-01-02"))
	fmt.Scanln(&input_start)

	if len(input_start) != 0 {
		if len(input_start) <= 2 {
			date, err := time.ParseInLocation("2006-01-02", fmt.Sprintf("%s-%s", start_date.Format("2006-01"), input_start), start_date.Location())
			if err != nil {
				log.Fatal(err)
			}
			start_date = findStartDate(date)
		} else {
			date, err := time.ParseInLocation("2006-01-02", input_start, start_date.Location())
			if err != nil {
				log.Fatal(err)
			}
			start_date = findStartDate(date)
		}
	}

	// Same thing for the end date. We default to the Sunday of the following week.
	// If only 2 digits are entered, we assume they mean this year and month
	// otherwise they will need to enter the full YYYY-MM-DD date in order to set
	// the end date.
	fmt.Printf("End Date (%s): ", end_date.Format("2006-01-02"))
	fmt.Scanln(&input_end)

	if len(input_end) != 0 {
		if len(input_end) <= 2 {
			date, err := time.ParseInLocation("2006-01-02", fmt.Sprintf("%s-%s", end_date.Format("2006-01"), input_end), end_date.Location())
			if err != nil {
				log.Fatal(err)
			}
			end_date = findEndDate(date)
		} else {
			date, err := time.ParseInLocation("2006-01-02", input_end, end_date.Location())
			if err != nil {
				log.Fatal(err)
			}
			end_date = findEndDate(date)
		}
	}

	// We now need to loop through the start/end range to determine how many
	// weeks we are reporting on.
	weeks := make([]Week, 0)
	d := start_date
	for d.Format("2006-01-02") != end_date.Format("2006-01-02") {
		if d.Weekday() == time.Monday {
			weeks = append(weeks, Week{Start: d, End: findEndDate(d), Of: d.Format("1/2/2006")})
		}
		d = d.AddDate(0, 0, +1)
	}

	// Loop over weeks to get report data from Harvest and append

	for i, week := range weeks {
		service, err := spreadsheet.NewService()
		if err != nil {
			log.Fatalf("cannot get spreadsheet 1: %v", err)
		}

		spreadsheet, err := service.FetchSpreadsheet(config.SpreadsheetID)
		if err != nil {
			log.Fatalf("cannot get spreadsheet 2: %v", err)
		}

		sheet, err := spreadsheet.SheetByIndex(0)
		if err != nil {
			log.Fatalf("cannot get spreadsheet 3: %v", err)
		}

		lastRow := len(sheet.Rows) - 1
		pov := sheet.Rows[lastRow][7].Value
		previousOverUnder, err := strconv.ParseFloat(pov, 64)
		if err != nil {
			previousOverUnder = float64(0)
		}

		fmt.Printf("PTO/Holiday/Sick leave for week of %s: ", week.Start.Format("Jan _2 2006"))
		fmt.Scanln(&weeks[i].Pto)
		fmt.Println("")

		weeks[i].BillableEntries = getTimeEntries(week.Start, week.End, true)
		weeks[i].NonBillableEntries = getTimeEntries(week.Start, week.End, false)
		calculateHours(&weeks[i])

		fmt.Printf("Week of %s\n", week.Start.Format("Jan _2 2006"))
		fmt.Println("------------")
		for _, e := range weeks[i].BillableEntries {
			lastRow++
			sheet.Update(lastRow, 0, weeks[i].Of)
			sheet.Update(lastRow, 1, config.Employee)
			sheet.Update(lastRow, 2, config.Category)
			sheet.Update(lastRow, 3, strconv.FormatFloat(e.DayEntry.Hours, 'f', 2, 64))
			sheet.Update(lastRow, 4, e.DayEntry.Notes)

			fmt.Printf("%.2f | %s\n", e.DayEntry.Hours, e.DayEntry.Notes)
		}
		fmt.Println("----")
		total := weeks[i].BillableHours + weeks[i].NonBillableHours + weeks[i].Pto
		overUnder := (total + previousOverUnder) - float64(37.5)

		sheet.Update(lastRow, 7, strconv.FormatFloat(overUnder, 'f', 2, 64))
		sheet.Update(lastRow, 8, strconv.FormatFloat(weeks[i].Pto, 'f', 2, 64))
		sheet.Update(lastRow, 9, strconv.FormatFloat(weeks[i].NonBillableHours, 'f', 2, 64))
		err = sheet.Synchronize()
		if err != nil {
			log.Fatalf("cannot save spreadsheet data: %v", err)
		}

		fmt.Printf("%.2f | %.2f Total (%.2f Non billable hours, %.2f PTO, %.2f Over/Under)\n", weeks[i].BillableHours, total, weeks[i].NonBillableHours, weeks[i].Pto, overUnder)
		fmt.Println("")
	}

}
