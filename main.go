package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	statsURL = "http://srv.msk01.gigacorp.local/_stats"

	pollInterval = 10 * time.Second

	maxConsecutiveErrors = 3

	loadAvgThreshold   = 30.0
	memUsageThreshold  = 0.80
	diskUsageThreshold = 0.90
	netUsageThreshold  = 0.90
)

type ServerStats struct {
	LoadAvg   float64
	TotalMem  int64
	UsedMem   int64
	TotalDisk int64
	UsedDisk  int64
	TotalNet  int64
	UsedNet   int64
}

func main() {
	consecutiveErrors := 0

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	log.Printf("Запуск мониторинга сервера: %s (интервал: %s)\n", statsURL, pollInterval)

	for {
		stats, err := fetchAndParseStats(client, statsURL)

		if err != nil {
			fmt.Fprintf(os.Stderr, "Ошибка (попытка %d): %v\n", consecutiveErrors+1, err)
			consecutiveErrors++
			if consecutiveErrors >= maxConsecutiveErrors {
				fmt.Println("Unable to fetch server statistic.")
			}
		} else {
			consecutiveErrors = 0
			checkMetrics(stats)
		}

		time.Sleep(pollInterval)
	}
}

func fetchAndParseStats(client *http.Client, url string) (*ServerStats, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("ошибка HTTP GET: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("получен некорректный статус: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения тела ответа: %w", err)
	}

	dataStr := strings.TrimSpace(string(body))
	parts := strings.Split(dataStr, ",")

	if len(parts) != 7 {
		return nil, fmt.Errorf("неверный формат данных: ожидалось 7 частей, получено %d", len(parts))
	}

	parseFloat := func(s string) (float64, error) {
		return strconv.ParseFloat(strings.TrimSpace(s), 64)
	}
	parseInt := func(s string) (int64, error) {
		return strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	}

	stats := &ServerStats{}

	if stats.LoadAvg, err = parseFloat(parts[0]); err != nil {
		return nil, fmt.Errorf("ошибка парсинга Load Average: %w", err)
	}
	if stats.TotalMem, err = parseInt(parts[1]); err != nil {
		return nil, fmt.Errorf("ошибка парсинга Total Memory: %w", err)
	}
	if stats.UsedMem, err = parseInt(parts[2]); err != nil {
		return nil, fmt.Errorf("ошибка парсинга Used Memory: %w", err)
	}
	if stats.TotalDisk, err = parseInt(parts[3]); err != nil {
		return nil, fmt.Errorf("ошибка парсинга Total Disk: %w", err)
	}
	if stats.UsedDisk, err = parseInt(parts[4]); err != nil {
		return nil, fmt.Errorf("ошибка парсинга Used Disk: %w", err)
	}
	if stats.TotalNet, err = parseInt(parts[5]); err != nil {
		return nil, fmt.Errorf("ошибка парсинга Total Network: %w", err)
	}
	if stats.UsedNet, err = parseInt(parts[6]); err != nil {
		return nil, fmt.Errorf("ошибка парсинга Used Network: %w", err)
	}

	return stats, nil
}

func checkMetrics(stats *ServerStats) {
	if stats.LoadAvg > loadAvgThreshold {
		fmt.Printf("Load Average is too high: %g\n", stats.LoadAvg)
	}

	if stats.TotalMem > 0 {
		memUsage := float64(stats.UsedMem) / float64(stats.TotalMem)
		if memUsage > memUsageThreshold {
			fmt.Printf("Memory usage too high: %.0f%%\n", memUsage*100)
		}
	}

	if stats.TotalDisk > 0 {
		diskUsage := float64(stats.UsedDisk) / float64(stats.TotalDisk)
		if diskUsage > diskUsageThreshold {
			freeDiskBytes := stats.TotalDisk - stats.UsedDisk
			freeDiskMb := freeDiskBytes / 1048576
			fmt.Printf("Free disk space is too low: %d Mb left\n", freeDiskMb)
		}
	}

	if stats.TotalNet > 0 {
		netUsage := float64(stats.UsedNet) / float64(stats.TotalNet)
		if netUsage > netUsageThreshold {
			freeNetBps := stats.TotalNet - stats.UsedNet
			freeNetMbps := float64(freeNetBps*8) / 1_000_000.0
			fmt.Printf("Network bandwidth usage high: %.2f Mbit/s available\n", freeNetMbps)
		}
	}
}
