package turbostat

import (
	"bufio"
	"log"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
	"sync"
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/inputs"
)

// MockPlugin struct should be named the same as the Plugin
type Turbostat struct {
	Tinterval string `toml:"tinterval"`
  Hide []string `toml:hide`
  Tpath string `toml:tpath`
	mux sync.Mutex
}

var interval string = "1"
var ready bool = false
var turbo *exec.Cmd
var scan *bufio.Scanner
var outputs []*Output

type Output struct {
	time        time.Time
	time_str    string
	header      []string
	summary_row []string
	main_rows   [][]string
	is_complete bool
}

// Description will appear directly above the plugin definition in the config file
func (m *Turbostat) Description() string {
	return `Use output of turbostat as metrics`
}

// SampleConfig will populate the sample configuration portion of the plugin's configuration
func (m *Turbostat) SampleConfig() string {
	return `
  # Turbostat collection interval in seconds
  tinterval = 1
  # Attributes to hide
  hide = ["RAM_%", "PKG_%", "CPUGFX%", "POLL", "SMI", "IRQ", "X2APIC", "APIC", "POLL%"]
  # Telegraf Path
  tpath = "turbostat"
  `
}

func parse(val string) {
	r := regexp.MustCompile(`\s`)
	var row []string = strings.Split(r.ReplaceAllString(strings.TrimSpace(val), ","), ",")
	if row[0] == "usec" {
		if len(outputs) > 0 {
			outputs[len(outputs)-1].is_complete = true
		}

		// begin new output
		o := &Output{}
		o.header = make([]string, 0)
		o.summary_row = make([]string, 0)
		o.main_rows = make([][]string, 0)
		o.is_complete = false

		outputs = append(outputs, o)
		// first row
		// save header
		for j := 0; j < len(row); j++ {
			outputs[len(outputs)-1].header = append(outputs[len(outputs)-1].header, row[j])
		}
	} else if len(outputs[len(outputs)-1].header) > 0 && (outputs[len(outputs)-1].summary_row == nil || len(outputs[len(outputs)-1].summary_row) == 0) {
		// second row
		// save summary row
		for j := 0; j < len(row); j++ {
			outputs[len(outputs)-1].summary_row = append(outputs[len(outputs)-1].summary_row, row[j])
		}

	} else {
		outputs[len(outputs)-1].main_rows = append(outputs[len(outputs)-1].main_rows, make([]string, 0))
		// all other rows
		// save the row
		for j := 0; j < len(row); j++ {
			outputs[len(outputs)-1].main_rows[len(outputs[len(outputs)-1].main_rows)-1] = append(outputs[len(outputs)-1].main_rows[len(outputs[len(outputs)-1].main_rows)-1], row[j])
		}
	}
}

func collect(m *Turbostat) {
	for scan.Scan() {
		m.mux.Lock()
		parse(scan.Text())
		m.mux.Unlock()
	}
}

func getTime(t string) time.Time {
	var parts []string = strings.Split(t, ".")
	sec, _ := strconv.ParseInt(parts[0], 10, 64)
	nsec, _ := strconv.ParseInt(parts[1], 10, 64)
	return time.Unix(sec, nsec)
}

// Gather defines what data the plugin will gather.
func (m *Turbostat) Gather(acc telegraf.Accumulator) error {
	if !ready {
    turbo_args := []string{"--quiet", "--interval", m.Tinterval, "--enable", "all"}
    if m.Hide != nil && len(m.Hide) > 0 {

			str := ""

      for i, a := range m.Hide {
				if i != 0 {
					str += ","
				}
				str += a
      }
			turbo_args = append(turbo_args, "--hide")
			turbo_args = append(turbo_args, str)
    }

		turbo = exec.Command(m.Tpath, turbo_args...)

		outPipe, _ := turbo.StdoutPipe()
		scan = bufio.NewScanner(outPipe)

		err_pipe, _ := turbo.StderrPipe()
		err_scan := bufio.NewScanner(err_pipe)

		err := turbo.Start()
		if err != nil {
			for err_scan.Scan() {
				log.Println(err_scan.Text())
			}
			return nil
		}
		ready = true
		go collect(m)
	} else {

		// for all gathered outputs
		m.mux.Lock()
		defer m.mux.Unlock()

		for i := 0; i < len(outputs); i++ {
			o := outputs[i]

			if o.is_complete {
				summary_time := getTime(o.summary_row[1])

				tags := map[string]string{
					"summary":   "true",
					"phys_core": "N/A",
					"virt_core": "N/A",
				}

				fields := make(map[string]interface{})
				fval, _ := strconv.ParseFloat(o.summary_row[0], 64)
				fields["collect_duration_usec"] = fval

				for j := 4; j < len(o.summary_row); j++ {
					fval, _ = strconv.ParseFloat(o.summary_row[j], 64)
					fields[o.header[j]] = fval
				}

				acc.AddFields("turbostat", fields, tags, summary_time)

				for j := 0; j < len(o.main_rows); j++ {

					row_time := getTime(o.main_rows[j][1])

					tags = map[string]string{
						"summary":   "false",
						"phys_core": o.main_rows[j][2],
						"virt_core": o.main_rows[j][3],
					}

					fields = make(map[string]interface{})
					fval, _ = strconv.ParseFloat(o.main_rows[j][0], 64)
					fields["collect_duration_usec"] = fval

					for k := 0; k < len(o.main_rows[j]); k++ {
						fval, _ = strconv.ParseFloat(o.main_rows[j][k], 64)
						fields[o.header[k]] = fval
					}

					acc.AddFields("turbostat", fields, tags, row_time)
				}

				tmp := outputs[:i]
				tmp = append(tmp, outputs[i + 1:]...)
				outputs = tmp

			} else {
				continue
			}
		}

	}
	return nil
}

func init() {
	inputs.Add("turbostat", func() telegraf.Input { return &Turbostat{Tpath: "turbostat", Tinterval: "1", Hide: []string{"RAM_%", "PKG_%", "CPUGFX%", "POLL", "SMI", "IRQ", "X2APIC", "APIC", "POLL%"}} })
}
