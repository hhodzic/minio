/*
 * MinIO Cloud Storage, (C) 2020 MinIO, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package cmd

import (
	"context"
	"net/http"
	"os"
	"sync"
	"syscall"

	"github.com/minio/minio/pkg/disk"
	"github.com/minio/minio/pkg/madmin"
	cpuhw "github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/host"
	memhw "github.com/shirou/gopsutil/mem"
	"github.com/shirou/gopsutil/process"
)

func getLocalCPUOBDInfo(ctx context.Context) madmin.ServerCPUOBDInfo {
	addr := ""
	if globalIsDistXL {
		addr = GetLocalPeer(globalEndpoints)
	} else {
		addr = "minio"
	}

	info, err := cpuhw.InfoWithContext(ctx)
	if err != nil {
		return madmin.ServerCPUOBDInfo{
			Addr:  addr,
			Error: err.Error(),
		}
	}

	time, err := cpuhw.TimesWithContext(ctx, false)
	if err != nil {
		return madmin.ServerCPUOBDInfo{
			Addr:  addr,
			Error: err.Error(),
		}
	}

	return madmin.ServerCPUOBDInfo{
		Addr:     addr,
		CPUStat:  info,
		TimeStat: time,
		Error:    "",
	}

}

func getLocalDrivesOBD(ctx context.Context, parallel bool, endpointZones EndpointZones, r *http.Request) madmin.ServerDrivesOBDInfo {
	var drivesOBDInfo []madmin.DriveOBDInfo
	wg := sync.WaitGroup{}
	for _, ep := range endpointZones {
		for i, endpoint := range ep.Endpoints {
			// Only proceed for local endpoints
			if endpoint.IsLocal {
				if _, err := os.Stat(endpoint.Path); err != nil {
					// Since this drive is not available, add relevant details and proceed
					drivesOBDInfo = append(drivesOBDInfo, madmin.DriveOBDInfo{
						Path:  endpoint.Path,
						Error: err.Error(),
					})
					continue
				}
				measure := func(index int) {
					latency, throughput, err := disk.GetOBDInfo(ctx, pathJoin(endpoint.Path, minioMetaTmpBucket, mustGetUUID()))
					driveOBDInfo := madmin.DriveOBDInfo{
						Path:       endpoint.Path,
						Latency:    latency,
						Throughput: throughput,
					}
					if err != nil {
						driveOBDInfo.Error = err.Error()
					}
					drivesOBDInfo = append(drivesOBDInfo, driveOBDInfo)
					wg.Done()
				}
				wg.Add(1)

				if parallel {
					go measure(i)
				} else {
					measure(i)
				}
			}
		}
	}
	wg.Wait()
	addr := ""
	if globalIsDistXL {
		addr = GetLocalPeer(endpointZones)
	} else {
		addr = "minio"
	}
	if parallel {
		return madmin.ServerDrivesOBDInfo{
			Addr:     addr,
			Parallel: drivesOBDInfo,
		}
	}
	return madmin.ServerDrivesOBDInfo{
		Addr:   addr,
		Serial: drivesOBDInfo,
	}
}

func getLocalMemOBD(ctx context.Context) madmin.ServerMemOBDInfo {
	addr := ""
	if globalIsDistXL {
		addr = GetLocalPeer(globalEndpoints)
	} else {
		addr = "minio"
	}

	swap, err := memhw.SwapMemoryWithContext(ctx)
	if err != nil {
		return madmin.ServerMemOBDInfo{
			Addr:  addr,
			Error: err.Error(),
		}
	}

	vm, err := memhw.VirtualMemoryWithContext(ctx)
	if err != nil {
		return madmin.ServerMemOBDInfo{
			Addr:  addr,
			Error: err.Error(),
		}
	}

	return madmin.ServerMemOBDInfo{
		Addr:       addr,
		SwapMem:    swap,
		VirtualMem: vm,
		Error:      "",
	}
}

func getLocalProcOBD(ctx context.Context) madmin.ServerProcOBDInfo {
	addr := ""
	if globalIsDistXL {
		addr = GetLocalPeer(globalEndpoints)
	} else {
		addr = "minio"
	}

	errProcInfo := func(err error) madmin.ServerProcOBDInfo {
		return madmin.ServerProcOBDInfo{
			Addr:  addr,
			Error: err.Error(),
		}
	}

	selfPid := int32(syscall.Getpid())
	self, err := process.NewProcess(selfPid)
	if err != nil {
		return errProcInfo(err)
	}

	processes := []*process.Process{self}
	if err != nil {
		return errProcInfo(err)
	}

	sysProcs := []madmin.SysOBDProcess{}
	for _, proc := range processes {
		sysProc := madmin.SysOBDProcess{}
		sysProc.Pid = proc.Pid

		bg, err := proc.BackgroundWithContext(ctx)
		if err != nil {
			return errProcInfo(err)
		}
		sysProc.Background = bg

		cpuPercent, err := proc.CPUPercentWithContext(ctx)
		if err != nil {
			return errProcInfo(err)
		}
		sysProc.CPUPercent = cpuPercent

		children, _ := proc.ChildrenWithContext(ctx)

		for _, c := range children {
			sysProc.Children = append(sysProc.Children, c.Pid)
		}
		cmdLine, err := proc.CmdlineWithContext(ctx)
		if err != nil {
			return errProcInfo(err)
		}
		sysProc.CmdLine = cmdLine

		conns, err := proc.ConnectionsWithContext(ctx)
		if err != nil {
			return errProcInfo(err)
		}
		sysProc.Connections = conns

		createTime, err := proc.CreateTimeWithContext(ctx)
		if err != nil {
			return errProcInfo(err)
		}
		sysProc.CreateTime = createTime

		cwd, err := proc.CwdWithContext(ctx)
		if err != nil {
			return errProcInfo(err)
		}
		sysProc.Cwd = cwd

		exe, err := proc.ExeWithContext(ctx)
		if err != nil {
			return errProcInfo(err)
		}
		sysProc.Exe = exe

		gids, err := proc.GidsWithContext(ctx)
		if err != nil {
			return errProcInfo(err)
		}
		sysProc.Gids = gids

		ioCounters, err := proc.IOCountersWithContext(ctx)
		if err != nil {
			return errProcInfo(err)
		}
		sysProc.IOCounters = ioCounters

		isRunning, err := proc.IsRunningWithContext(ctx)
		if err != nil {
			return errProcInfo(err)
		}
		sysProc.IsRunning = isRunning

		memInfo, err := proc.MemoryInfoWithContext(ctx)
		if err != nil {
			return errProcInfo(err)
		}
		sysProc.MemInfo = memInfo

		memMaps, err := proc.MemoryMapsWithContext(ctx, true)
		if err != nil {
			return errProcInfo(err)
		}
		sysProc.MemMaps = memMaps

		memPercent, err := proc.MemoryPercentWithContext(ctx)
		if err != nil {
			return errProcInfo(err)
		}
		sysProc.MemPercent = memPercent

		name, err := proc.NameWithContext(ctx)
		if err != nil {
			return errProcInfo(err)
		}
		sysProc.Name = name

		netIOCounters, err := proc.NetIOCountersWithContext(ctx, false)
		if err != nil {
			return errProcInfo(err)
		}
		sysProc.NetIOCounters = netIOCounters

		nice, err := proc.NiceWithContext(ctx)
		if err != nil {
			return errProcInfo(err)
		}
		sysProc.Nice = nice

		numCtxSwitches, err := proc.NumCtxSwitchesWithContext(ctx)
		if err != nil {
			return errProcInfo(err)
		}
		sysProc.NumCtxSwitches = numCtxSwitches

		numFds, err := proc.NumFDsWithContext(ctx)
		if err != nil {
			return errProcInfo(err)
		}
		sysProc.NumFds = numFds

		numThreads, err := proc.NumThreadsWithContext(ctx)
		if err != nil {
			return errProcInfo(err)
		}
		sysProc.NumThreads = numThreads

		openFiles, err := proc.OpenFilesWithContext(ctx)
		if err != nil {
			return errProcInfo(err)
		}
		sysProc.OpenFiles = openFiles

		pageFaults, err := proc.PageFaultsWithContext(ctx)
		if err != nil {
			return errProcInfo(err)
		}
		sysProc.PageFaults = pageFaults

		parent, err := proc.ParentWithContext(ctx)
		if err != nil {
			return errProcInfo(err)
		}
		sysProc.Parent = parent.Pid

		ppid, err := proc.PpidWithContext(ctx)
		if err != nil {
			return errProcInfo(err)
		}
		sysProc.Ppid = ppid

		rlimit, err := proc.RlimitWithContext(ctx)
		if err != nil {
			return errProcInfo(err)
		}
		sysProc.Rlimit = rlimit

		status, err := proc.StatusWithContext(ctx)
		if err != nil {
			return errProcInfo(err)
		}
		sysProc.Status = status

		tgid, err := proc.Tgid()
		if err != nil {
			return errProcInfo(err)
		}
		sysProc.Tgid = tgid

		threads, err := proc.ThreadsWithContext(ctx)
		if err != nil {
			return errProcInfo(err)
		}
		sysProc.Threads = threads

		times, err := proc.TimesWithContext(ctx)
		if err != nil {
			return errProcInfo(err)
		}
		sysProc.Times = times

		uids, err := proc.UidsWithContext(ctx)
		if err != nil {
			return errProcInfo(err)
		}
		sysProc.Uids = uids

		username, err := proc.UsernameWithContext(ctx)
		if err != nil {
			return errProcInfo(err)
		}
		sysProc.Username = username

		sysProcs = append(sysProcs, sysProc)
	}

	return madmin.ServerProcOBDInfo{
		Addr:      addr,
		Processes: sysProcs,
		Error:     "",
	}
}

func getLocalOsInfoOBD(ctx context.Context) madmin.ServerOsOBDInfo {
	addr := ""
	if globalIsDistXL {
		addr = GetLocalPeer(globalEndpoints)
	} else {
		addr = "minio"
	}

	info, err := host.InfoWithContext(ctx)
	if err != nil {
		return madmin.ServerOsOBDInfo{
			Addr:  addr,
			Error: err.Error(),
		}
	}

	sensors, err := host.SensorsTemperaturesWithContext(ctx)
	if err != nil {
		return madmin.ServerOsOBDInfo{
			Addr:  addr,
			Error: err.Error(),
		}
	}

	users, err := host.UsersWithContext(ctx)
	if err != nil {
		return madmin.ServerOsOBDInfo{
			Addr:  addr,
			Error: err.Error(),
		}
	}

	return madmin.ServerOsOBDInfo{
		Addr:    addr,
		Info:    info,
		Sensors: sensors,
		Users:   users,
		Error:   "",
	}
}
