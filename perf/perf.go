package perf

import (
	"bufio"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
	"sync"
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/inputs"
)

// MockPlugin struct should be named the same as the Plugin
type Perf struct {
	Pinterval string `toml:"pinterval"`
	mux sync.Mutex
}

var interval string = "1000"
var ready bool = false
var perf *exec.Cmd
var scan *bufio.Scanner
var outputs []*Output
var last_collection_period float64 = 0

type Output struct {
	collection_time        time.Time
	sep_collection_periods []float64
	fields                 map[string]interface{}
	is_complete 					 bool
}

// Description will appear directly above the plugin definition in the config file
func (m *Perf) Description() string {
	return `Use output of turbostat as metrics`
}

// SampleConfig will populate the sample configuration portion of the plugin's configuration
func (m *Perf) SampleConfig() string {
	return `
  # Perf collection interval in milliseconds
  pinterval = 1000
  `
}

func parse(val string) {
	var row []string = strings.Split(strings.TrimSpace(val), "|")
	if row[2] != "" {
		if len(outputs) > 0 {
			outputs[len(outputs) -1].is_complete = true
		}
		// first row - new output
		o := &Output{}
		o.sep_collection_periods = make([]float64, 0)
		o.fields = make(map[string]interface{})
		o.is_complete = false
		outputs = append(outputs, o)

		collection_period, _ := strconv.ParseFloat(row[0], 64)
		outputs[len(outputs)-1].sep_collection_periods = append(outputs[len(outputs)-1].sep_collection_periods, collection_period)

		count, _ := strconv.ParseFloat(row[1], 64)
		outputs[len(outputs)-1].fields[row[3]+"_"+row[2]] = count

		more, _ := strconv.ParseFloat(row[6], 64)
		outputs[len(outputs)-1].fields[row[3]+"_"+strings.Replace(row[7], " ", "_", -1)] = more
		o.collection_time = time.Now()

	} else {
		// other rows
		if row[1] == "<not supported>" {
			return
		} else {
			collection_period, _ := strconv.ParseFloat(row[0], 64)
			outputs[len(outputs)-1].sep_collection_periods = append(outputs[len(outputs)-1].sep_collection_periods, collection_period)

			count, _ := strconv.ParseFloat(row[1], 64)
			outputs[len(outputs)-1].fields[row[3]] = count

			more, _ := strconv.ParseFloat(row[6], 64)
			if row[7] != "" {
				outputs[len(outputs)-1].fields[row[3]+"_"+strings.Replace(row[7], " ", "_", -1)] = more
			}
		}
	}
}

func collect(m *Perf) {
	for scan.Scan() {
		m.mux.Lock()
		parse(scan.Text())
		m.mux.Unlock()
	}
}

// Gather defines what data the plugin will gather.
func (m *Perf) Gather(acc telegraf.Accumulator) error {
	if !ready {

		perf_args := []string{"stat", "-x", "|", "-I", m.Pinterval, "--interval-count", "0", "-d", "-d", "-a"}
		perf = exec.Command("perf", perf_args...)

		// so weird - perf outputs to stderr for stat
		err_pipe, _ := perf.StderrPipe()
		scan = bufio.NewScanner(err_pipe)

		err := perf.Start()
		if err != nil {
			fmt.Println(err)
			return nil
		}

		ready = true

		go collect(m)

	} else {

		for i := 0; i < len(outputs); i++ {
			o := outputs[i]

			if o.is_complete {
				var tot float64 = 0

				for _, value := range o.sep_collection_periods {
					tot += value
				}
				avg_col_p := tot / float64(len(o.sep_collection_periods))

				current_collection_period := avg_col_p - last_collection_period

				last_collection_period = avg_col_p

				tags := make(map[string]string)

				fields := o.fields

				fields["collection_period"] = current_collection_period

				acc.AddFields("perf", fields, tags, o.collection_time)

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
	inputs.Add("perf", func() telegraf.Input { return &Perf{Pinterval: "1000"} })
}
