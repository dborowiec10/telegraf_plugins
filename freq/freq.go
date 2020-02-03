package freq

import (
  "github.com/influxdata/telegraf"
  "github.com/influxdata/telegraf/plugins/inputs"
  "log"
  "os"
  "os/exec"
  "bufio"
  "strings"
  "strconv"
)

type Freq struct {}

type Cpu struct {
  virt_core_id int
  phys_core_id int
  frequency int
  min_frequency int
  max_frequency int
}

var cpu_count int

var cpus []Cpu

var FreqConfig = `
  ## Nothing to configure
`

func (s *Freq) SampleConfig() string {
  return FreqConfig
}

func (s *Freq) Description() string {
  return "Bleh"
}

func (s *Freq) Gather(acc telegraf.Accumulator) error {

  for _, c := range cpus {
    cpu := "-c " + strconv.Itoa(c.virt_core_id)
    args_cpu := []string{cpu, "-f"}
    out_cpu, err_cpu := exec.Command("cpufreq-info", args_cpu...).Output()
    if err_cpu != nil {
      log.Fatal(err_cpu)
    }
    out := strings.Trim(string(out_cpu), " \n")
    out_c, err := strconv.Atoi(out)
    if err != nil {
      log.Fatal(err)
    }

    c.frequency = out_c

    tags := map[string]string {
      "virt_core_id": strconv.Itoa(c.virt_core_id),
      "phys_core_id": strconv.Itoa(c.phys_core_id),
    }

    fields := map[string]interface{}{
      "min_frequency": c.min_frequency,
      "max_frequency": c.max_frequency,
      "frequency": c.frequency,
    }

    acc.AddFields("freq", fields, tags)
  }

  return nil
}

func init(){
  inputs.Add("freq", func() telegraf.Input { return &Freq{} })

  file, err := os.Open("/proc/cpuinfo")
  if err != nil {
    log.Fatal(err)
  }
  defer file.Close()

  var buf string
  var virt_c_id int
  var phys_c_id int

  args_cpu := []string{"-l"}
  out_cpu, err_cpu := exec.Command("cpufreq-info", args_cpu...).Output()
  if err_cpu != nil {
    log.Fatal(err_cpu)
  }

  var minmax []string = strings.Split(string(out_cpu), " ")

  out_min, err := strconv.Atoi(strings.Trim(minmax[0], "\n "))
  if err != nil {
    log.Fatal(err)
  }

  out_max, err := strconv.Atoi(strings.Trim(minmax[1], "\n "))
  if err != nil {
    log.Fatal(err)
  }

  scanner := bufio.NewScanner(file)
  for scanner.Scan() {
    buf = scanner.Text()
    if strings.Contains(buf, "processor") {
      vc_id, err := strconv.Atoi(strings.Trim(strings.Split(buf, ": ")[1], " "))
      if err != nil {
        log.Fatal(err)
      }
      virt_c_id = vc_id
    }

    if strings.Contains(buf, "core id"){
      pc_id, err := strconv.Atoi(strings.Trim(strings.Split(buf, ": ")[1], " "))
      if err != nil {
        log.Fatal(err)
      }
      phys_c_id = pc_id
    }

    if buf == "" {
      cpu_count++

      cpus = append(cpus, Cpu{virt_core_id: virt_c_id, phys_core_id: phys_c_id, min_frequency: out_min, max_frequency: out_max})
    }
  }

  if err := scanner.Err(); err != nil {
    log.Fatal(err)
  }

}
