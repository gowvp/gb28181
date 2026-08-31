package stat

import (
	"log/slog"
	"time"

	"github.com/ixugo/goddd/pkg/orm"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
)

type UsageStat struct {
	Name      string `json:"name"`       // 路径
	Unit      string `json:"unit"`       // 单位
	Size      string `json:"size"`       // 大小
	FreeSpace string `json:"free_space"` // 可用空间
	Used      string `json:"used"`       // 已使用
	Percent   string `json:"percent"`    // 比率
	Threshold string `json:"threshold"`  // 阈值
}

const (
	TopQueneCap = 30
)

var (
	memData = NewCircleQueue(TopQueneCap)
	cpuData = NewCircleQueue(TopQueneCap)
	// netUpData         = NewCircleQueue(TopQueneCap)
	netData           = NewCircleQueue(TopQueneCap)
	currentMem        float64
	currentCPU        float64
	currentMainDisk   uint64
	totalMainDisk     uint64
	currentKernelDisk float64
	totalKernelDisk   uint64
)

func GetCurrentMem() float64 {
	return currentMem
}

func GetCurrentCPU() float64 {
	return currentCPU
}

func GetCurrentMainDisk() uint64 {
	return currentMainDisk
}

func GetTotalMainDisk() uint64 {
	return totalMainDisk
}

func GetCurrentKernelDisk() float64 {
	return currentKernelDisk
}

func GetTotalKernelDisk() uint64 {
	return totalKernelDisk
}

func GetMemData() []PercentData {
	return memData.Range()
}

func GetCPUData() []PercentData {
	return cpuData.Range()
}

func GetNetData() []PercentData {
	return netData.Range()
}

// LoadTop 定时采集 CPU/内存/网络/磁盘指标，写入环形队列供前端轮询。
// 缓存前一次网络计数器计算速率，避免每轮双次 IOCounters + 1s 阻塞。
func LoadTop(path string) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	var prevNet []net.IOCountersStat
	var prevNetTime time.Time

	for range ticker.C {
		now := orm.Now()

		if cpuPercent, err := cpu.Percent(0, false); err != nil && err.Error() != "not implemented yet" {
			slog.Error("LoadTop cpu", "err", err)
		} else if len(cpuPercent) > 0 {
			cpuData.Push(PercentData{Time: now, Used: cpuPercent[0]})
			currentCPU = cpuPercent[0]
		}

		if memStat, err := mem.VirtualMemory(); err != nil && err.Error() != "not implemented yet" {
			slog.Error("LoadTop VirtualMemory", "err", err)
		} else if memStat != nil {
			memData.Push(PercentData{Time: now, Used: memStat.UsedPercent})
			currentMem = memStat.UsedPercent
		}

		if currNet, err := net.IOCounters(false); err == nil && len(currNet) > 0 {
			if len(prevNet) > 0 {
				elapsed := time.Since(prevNetTime).Seconds()
				if elapsed > 0 {
					netData.Push(PercentData{
						Time: now,
						Up:   float64(currNet[0].BytesSent-prevNet[0].BytesSent) / elapsed * 8,
						Down: float64(currNet[0].BytesRecv-prevNet[0].BytesRecv) / elapsed * 8,
					})
				}
			}
			prevNet = currNet
			prevNetTime = time.Now()
		}

		if diskres, err := disk.Usage(path); err == nil {
			currentMainDisk = diskres.Used
			totalMainDisk = diskres.Total
		}
	}
}
