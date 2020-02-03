package nhddtemp

import (
	"os/exec"
	"strconv"
	"strings"
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/inputs"
)

type Nhddtemp struct {
	Devices []string
}

var NhddtempConfig = `
  ## Nothing to configure
  devices = ["/dev/sda", "/dev/sdb"]
`

// Description will appear directly above the plugin definition in the config file
func (m *Nhddtemp) Description() string {
	return `Plugin collects hdd temperature`
}

func (s *Nhddtemp) SampleConfig() string {
	return NhddtempConfig
}

func (m *Nhddtemp) Gather(acc telegraf.Accumulator) error {

	for _, d := range m.Devices {
		args := []string{d}
		out, err := exec.Command("hddtemp", args...).Output()
    if err != nil {
      continue
    }

		x := strings.Split(string(out), ":")

		dev := strings.Replace(x[0], " ", "", -1)
		model := strings.Replace(x[1], " ", "", -1)
		x[2] = strings.Replace(x[2], " ", "", -1)
		runes := []rune(x[2])
		t := string(runes[:len(runes)-3])

		tags := map[string]string{
			"device": dev,
			"model":  model,
		}

		temp, e := strconv.Atoi(t)
    if e != nil {
      continue
    }

		fields := map[string]interface{}{
			"temperature": temp,
		}

		acc.AddFields("nhddtemp", fields, tags)

	}
  
	return nil
}

func init() {
	inputs.Add("nhddtemp", func() telegraf.Input { return &Nhddtemp{Devices: []string{"/dev/sda", "/dev/sdb"}} })
}
