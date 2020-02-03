package rpithermal

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/inputs"
)

type Rpithermal struct {
	Sinterval int `toml:"sinterval"`
}

var sensor_names []string

var ready bool = false

type Sensor struct {
	name    string
}

type Output struct {
	col_time    time.Time
	sensor_id   string
	temperature float64
}

var outputs []*Output
var sensors []*Sensor

// Description will appear directly above the plugin definition in the config file
func (m *Rpithermal) Description() string {
	return `Use output of raspberry pi thermal sensors`
}

// SampleConfig will populate the sample configuration portion of the plugin's configuration
func (m *Rpithermal) SampleConfig() string {
	return `
  ## scan interval in seconds
	sinterval = 1
  `
}

func collect(interval int) {
	for {
		for _, sensor := range sensors {

			file, err := os.Open("/sys/bus/w1/devices/" + sensor.name + "/w1_slave")
			if err != nil {
				log.Println(err)
			}

			scanner := bufio.NewScanner(file)

			scanner.Scan()
			if err := scanner.Err(); err != nil {
				log.Fatal(err)
			}
			scanner.Text()

			scanner.Scan()
			if err := scanner.Err(); err != nil {
				log.Fatal(err)
			}
			second_line := scanner.Text()

			if len(second_line) > 1 {

				temp := strings.Split(strings.TrimSpace(second_line), " ")

				t := temp[len(temp)-1]

				temp_float, _ := strconv.ParseFloat(strings.Split(t, "=")[1], 64)

				temp_celsius := temp_float / 1000

				outputs = append(outputs, &Output{col_time: time.Now(), sensor_id: sensor.name, temperature: temp_celsius})

			}

			file.Close()

		}

		time.Sleep(time.Duration(interval) * time.Second)

	}
}

// Gather defines what data the plugin will gather.
func (m *Rpithermal) Gather(acc telegraf.Accumulator) error {
	if !ready {
		go collect(m.Sinterval)
		ready = true
		return nil
	} else {
		for _, o := range outputs {
			tags := make(map[string]string)
			tags["sensor_id"] = o.sensor_id

			fields := make(map[string]interface{})

			fields["temperature_celcius"] = o.temperature

			acc.AddFields("rpithermal", fields, tags, o.col_time)
		}

		outputs = make([]*Output, 0)
	}
	return nil
}

func init() {
	inputs.Add("rpithermal", func() telegraf.Input { return &Rpithermal{Sinterval: 1} })

	files, err := ioutil.ReadDir("/sys/bus/w1/devices")
	if err != nil {
		log.Fatal(err)
	}

	for _, f := range files {
		if f.Name() != "w1_bus_master1" && f.Name() != "." && f.Name() != ".." {
			sensors = append(sensors, &Sensor{name: f.Name()})
		}
	}
}
