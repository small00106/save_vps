package reporter

import (
	"runtime"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
)

type Metrics struct {
	CPUPercent   float64 `json:"cpu_percent"`
	MemPercent   float64 `json:"mem_percent"`
	SwapUsed     int64   `json:"swap_used"`
	SwapTotal    int64   `json:"swap_total"`
	DiskTotal    int64   `json:"disk_total"`
	DiskUsed     int64   `json:"disk_used"`
	DiskPercent  float64 `json:"disk_percent"`
	Load1        float64 `json:"load1"`
	Load5        float64 `json:"load5"`
	Load15       float64 `json:"load15"`
	NetInSpeed   int64   `json:"net_in_speed"`
	NetOutSpeed  int64   `json:"net_out_speed"`
	NetInTotal   int64   `json:"net_in_total"`
	NetOutTotal  int64   `json:"net_out_total"`
	TCPConns     int     `json:"tcp_conns"`
	UDPConns     int     `json:"udp_conns"`
	ProcessCount int     `json:"process_count"`
	Uptime       int64   `json:"uptime"`
}

var (
	lastNetIn  uint64
	lastNetOut uint64
)

func Collect() *Metrics {
	m := &Metrics{}

	// CPU
	if percents, err := cpu.Percent(0, false); err == nil && len(percents) > 0 {
		m.CPUPercent = percents[0]
	}

	// Memory
	if v, err := mem.VirtualMemory(); err == nil {
		m.MemPercent = v.UsedPercent
	}
	if s, err := mem.SwapMemory(); err == nil {
		m.SwapUsed = int64(s.Used)
		m.SwapTotal = int64(s.Total)
	}

	// Disk
	rootPath := "/"
	if runtime.GOOS == "windows" {
		rootPath = "C:"
	}
	if d, err := disk.Usage(rootPath); err == nil {
		m.DiskTotal = int64(d.Total)
		m.DiskUsed = int64(d.Used)
		m.DiskPercent = d.UsedPercent
	}

	// Load
	if l, err := load.Avg(); err == nil {
		m.Load1 = l.Load1
		m.Load5 = l.Load5
		m.Load15 = l.Load15
	}

	// Network
	if counters, err := net.IOCounters(false); err == nil && len(counters) > 0 {
		currentIn := counters[0].BytesRecv
		currentOut := counters[0].BytesSent
		m.NetInTotal = int64(currentIn)
		m.NetOutTotal = int64(currentOut)

		if lastNetIn > 0 {
			m.NetInSpeed = int64(currentIn-lastNetIn) / 10  // per second (10s interval)
			m.NetOutSpeed = int64(currentOut-lastNetOut) / 10
		}
		lastNetIn = currentIn
		lastNetOut = currentOut
	}

	// Connections
	if conns, err := net.Connections("tcp"); err == nil {
		m.TCPConns = len(conns)
	}
	if conns, err := net.Connections("udp"); err == nil {
		m.UDPConns = len(conns)
	}

	// Process count
	if procs, err := process.Processes(); err == nil {
		m.ProcessCount = len(procs)
	}

	// Uptime
	if uptime, err := host.Uptime(); err == nil {
		m.Uptime = int64(uptime)
	}

	return m
}

func GetCPUModel() string {
	if infos, err := cpu.Info(); err == nil && len(infos) > 0 {
		return infos[0].ModelName
	}
	return "unknown"
}

func GetDiskTotal() (int64, error) {
	rootPath := "/"
	if runtime.GOOS == "windows" {
		rootPath = "C:"
	}
	d, err := disk.Usage(rootPath)
	if err != nil {
		return 0, err
	}
	return int64(d.Total), nil
}

func GetRAMTotal() int64 {
	if v, err := mem.VirtualMemory(); err == nil {
		return int64(v.Total)
	}
	return 0
}
