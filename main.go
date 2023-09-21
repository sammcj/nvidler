package main

import (
	"context"
	"flag"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

func main() {
	// Configuration with argument parsing
	var idleTimeThreshold int
	var warningOnly bool
	var targetWorkloads, whitelist string
	var logFile string
	var sleepInterval int
	var dockerEnabled bool

	flag.IntVar(&idleTimeThreshold, "idleTimeThreshold", 300, "Time threshold for idle GPUs in seconds")
	flag.BoolVar(&warningOnly, "warningOnly", true, "Warning only mode")
	flag.StringVar(&targetWorkloads, "targetWorkloads", "python,tensorflow,cuda,pytorch", "List of target workload process names (comma-separated)")
	flag.StringVar(&whitelist, "whitelist", "whitelisted_process,whitelisted_container,nvidia-smi,nvidler.sh", "Whitelisted processes and Docker containers (comma-separated)")
	flag.StringVar(&logFile, "logFile", "/var/log/gpu_idle_monitor.log", "Log file")
	flag.IntVar(&sleepInterval, "sleepInterval", 60, "Sleep interval in seconds")
	flag.BoolVar(&dockerEnabled, "docker", true, "Enable Docker container tracking")

	flag.Parse()

	// Convert comma-separated strings to slices
	targetWorkloadsSlice := strings.Split(targetWorkloads, ",")
	whitelistSlice := strings.Split(whitelist, ",")

	// Rotate and clean up old logs
	if _, err := os.Stat(logFile); err == nil {
		os.Rename(logFile, logFile+".1")
	}

	// Remove logs older than 7 days
	files, _ := os.ReadDir("/var/log/")
	for _, f := range files {
		if strings.HasPrefix(f.Name(), "gpu_idle_monitor.log.") {
			fileInfo, _ := os.Stat("/var/log/" + f.Name())
			if time.Since(fileInfo.ModTime()).Hours() > 7*24 {
				os.Remove("/var/log/" + f.Name())
			}
		}
	}

	// Initialize logger
	logFileHandle, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	defer logFileHandle.Close()

	multiWriter := io.MultiWriter(os.Stdout, logFileHandle)
	logger := log.New(multiWriter, "", log.LstdFlags)

	// Output the date and program settings
	currentDate := time.Now().Format("Mon Jan 2 15:04:05 2006")
	logger.Printf("Current Date: %s\n", currentDate)
	logger.Printf("Configuration: idleTimeThreshold=%d, warningOnly=%v, targetWorkloads=%v, whitelist=%v, logFile=%s, sleepInterval=%d, dockerEnabled=%v\n",
		idleTimeThreshold, warningOnly, targetWorkloadsSlice, whitelistSlice, logFile, sleepInterval, dockerEnabled)

	var cli *client.Client
	if dockerEnabled {
		var err error
		cli, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			logger.Println("Failed to initialize Docker client.")
			return
		}
	}

	logger.Println("Starting GPU idle monitor...")

	for {
		// Get GPU processes
		out, err := exec.Command("nvidia-smi", "--query-compute-apps=pid,used_memory", "--format=csv,noheader,nounits").Output()
		if err != nil {
			logger.Println("Failed to query GPU processes.")
			continue
		}

		gpuProcesses := strings.Split(strings.TrimSpace(string(out)), "\n")

		// Log GPU processes
		logger.Printf("Current GPU Processes:\n%s\n", strings.Join(gpuProcesses, "\n"))

		for _, process := range gpuProcesses {
			fields := strings.Split(process, ",")
			pidStr := strings.TrimSpace(fields[0])
			usedMemoryStr := strings.TrimSpace(fields[1])

			pid, _ := strconv.Atoi(pidStr)
			usedMemory, _ := strconv.Atoi(usedMemoryStr)

			// Get the process name
			out, err := exec.Command("ps", "-p", pidStr, "-o", "comm=").Output()
			if err != nil {
				logger.Printf("Failed to get process name for PID %d.\n", pid)
				continue
			}
			processName := strings.TrimSpace(string(out))

			// Get the Docker container name
			var dockerContainer string
			if dockerEnabled {
				containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{})
				if err != nil {
					logger.Println("Failed to get Docker container list.")
					continue
				}

				for _, container := range containers {
					inspect, err := cli.ContainerInspect(context.Background(), container.ID)
					if err != nil {
						logger.Printf("Failed to inspect container: %s\n", container.ID)
						continue
					}
					logger.Printf("nvidia-smi PID %s with Docker container PID: %d Name: %s\n", pidStr, inspect.State.Pid, strings.TrimPrefix(container.Names[0], "/"))
					if pidStr == strconv.Itoa(inspect.State.Pid) {
						dockerContainer = strings.TrimPrefix(container.Names[0], "/")
						break
					}
				}
			}

			// Check if the process name is in the target workloads list
			if contains(targetWorkloadsSlice, processName) {
				// Skip whitelisted processes and containers
				if contains(whitelistSlice, processName) || contains(whitelistSlice, dockerContainer) {
					continue
				}

				// If the used memory is zero, consider the process as idle
				if usedMemory == 0 {
					// Get the process start time
					out, err := exec.Command("ps", "-o", "lstart=", "-p", pidStr).Output()
					if err != nil {
						logger.Printf("Failed to get start time for PID %d.\n", pid)
						continue
					}
					startTimeStr := strings.TrimSpace(string(out))
					startTime, _ := time.Parse("Mon Jan 2 15:04:05 2006", startTimeStr)
					startTimeEpoch := startTime.Unix()

					// Get the current time
					currentTimeEpoch := time.Now().Unix()

					// Calculate the idle time
					idleTime := currentTimeEpoch - startTimeEpoch

					// If idle time is greater than the threshold, take action
					if idleTime > int64(idleTimeThreshold) {
						if warningOnly {
							logger.Printf("WARNING: Process %d (%s) in Docker container %s has been idle for more than %d seconds.\n", pid, processName, dockerContainer, idleTimeThreshold)
						} else {
							// Send a SIGTERM for graceful termination
							if err := exec.Command("kill", "-15", pidStr).Run(); err != nil {
								logger.Printf("Failed to send SIGTERM to PID %d.\n", pid)
								continue
							}
							logger.Printf("Terminated: Process %d (%s) in Docker container %s has been idle for more than %d seconds.\n", pid, processName, dockerContainer, idleTimeThreshold)
						}
					}
				}
			}
		}

		// Sleep for a minute before checking again
		time.Sleep(time.Duration(sleepInterval) * time.Second)
	}
}

// Helper function to check if a slice contains a string
func contains(slice []string, str string) bool {
	for _, v := range slice {
		if v == str {
			return true
		}
	}
	return false
}
