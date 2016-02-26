/*
Copyright 2016 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package helpers

import (
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/juju/ratelimit"
)

var (
	wg = &sync.WaitGroup{}
)

func newContainerName() string {
	return "benchmark_container_" + strconv.FormatInt(time.Now().UnixNano(), 10) + strconv.Itoa(rand.Int())
}

// CreateContainers creates num of containers
func CreateContainers(client *docker.Client, num int) []string {
	ids := []string{}
	for i := 0; i < num; i++ {
		name := newContainerName()
		dockerOpts := docker.CreateContainerOptions{
			Name: name,
			Config: &docker.Config{
				AttachStderr: false,
				AttachStdin:  false,
				AttachStdout: false,
				Tty:          true,
				Cmd:          []string{"/bin/bash"},
				Image:        "ubuntu",
			},
		}
		container, err := client.CreateContainer(dockerOpts)
		if err != nil {
			panic(fmt.Sprintf("Error create containers: %v", err))
		}
		ids = append(ids, container.ID)
	}
	return ids
}

// StartContainers starts all the containers in ids slice
func StartContainers(client *docker.Client, ids []string) {
	for _, id := range ids {
		client.StartContainer(id, &docker.HostConfig{})
	}
}

// StopContainers stops all the containers in ids slice
func StopContainers(client *docker.Client, ids []string) {
	for _, id := range ids {
		client.StopContainer(id, 10)
	}
}

// RemoveContainers removes all the containers in ids slice
func RemoveContainers(client *docker.Client, ids []string) {
	for _, id := range ids {
		removeOpts := docker.RemoveContainerOptions{
			ID: id,
		}
		if err := client.RemoveContainer(removeOpts); err != nil {
			panic(fmt.Sprintf("Error remove containers: %v", err))
		}
	}
}

// CreateDeadContainers creates num of containers but not starts them
func CreateDeadContainers(client *docker.Client, num int) []string {
	return CreateContainers(client, num)
}

// CreateAliveContainers creates num of containers and also starts them
func CreateAliveContainers(client *docker.Client, num int) []string {
	ids := CreateContainers(client, num)
	StartContainers(client, ids)
	return ids
}

// DoListContainerBenchmark does periodically ListContainers with specific interval, returns latencies of all the calls
func DoListContainerBenchmark(client *docker.Client, interval, testPeriod time.Duration, all bool, stopchan chan int) []int {
	startTime := time.Now()
	latencies := []int{}
	for {
		start := time.Now()
		client.ListContainers(docker.ListContainersOptions{All: all})
		end := time.Now()
		latencies = append(latencies, int(end.Sub(start).Nanoseconds()))
		if stopchan == nil {
			if time.Now().Sub(startTime) >= testPeriod {
				return latencies
			}
		} else {
			select {
			case <-stopchan:
				return latencies
			default:
			}
		}
		if interval != 0 {
			time.Sleep(interval)
		}
	}
	return latencies
}

// DoInspectContainerBenchmark does periodically InspectContainer with specific interval, returns latencies of all the calls
func DoInspectContainerBenchmark(client *docker.Client, interval, testPeriod time.Duration, containerIds []string) []int {
	startTime := time.Now()
	latencies := []int{}
	rand.Seed(time.Now().Unix())
	for {
		containerId := containerIds[rand.Int()%len(containerIds)]
		start := time.Now()
		client.InspectContainer(containerId)
		end := time.Now()
		latencies = append(latencies, int(end.Sub(start).Nanoseconds()))
		if time.Now().Sub(startTime) >= testPeriod {
			break
		}
		if interval != 0 {
			time.Sleep(interval)
		}
	}
	return latencies
}

// DoParalListContainerBenchmark starts routineNumber of goroutines and let them do DoListContainerBenchmark, returns latencies of all the calls
func DoParalListContainerBenchmark(client *docker.Client, interval, testPeriod time.Duration, routineNumber int, all bool) []int {
	wg.Add(routineNumber)
	latenciesTable := make([][]int, routineNumber)
	for i := 0; i < routineNumber; i++ {
		go func(index int) {
			latenciesTable[index] = DoListContainerBenchmark(client, interval, testPeriod, all, nil)
			wg.Done()
		}(i)
	}
	wg.Wait()
	allLatencies := []int{}
	for _, latencies := range latenciesTable {
		allLatencies = append(allLatencies, latencies...)
	}
	return allLatencies
}

// DoParalInspectContainerBenchmark starts routineNumber of goroutines and let them do DoInspectContainerBenchmark, returns latencies of all the calls
func DoParalInspectContainerBenchmark(client *docker.Client, interval, testPeriod time.Duration, routineNumber int, containerIds []string) []int {
	wg.Add(routineNumber)
	latenciesTable := make([][]int, routineNumber)
	for i := 0; i < routineNumber; i++ {
		go func(index int) {
			latenciesTable[index] = DoInspectContainerBenchmark(client, interval, testPeriod, containerIds)
			wg.Done()
		}(i)
	}
	wg.Wait()
	allLatencies := []int{}
	for _, latencies := range latenciesTable {
		allLatencies = append(allLatencies, latencies...)
	}
	return allLatencies
}

// DoParalContainerStartBenchmark starts routineNumber of goroutines and let them start containers, returns latencies of all the starting calls
// There is a global rate limit on starting calls per second.
func DoParalContainerStartBenchmark(client *docker.Client, qps float64, testPeriod time.Duration, routineNumber int) []int {
	wg.Add(routineNumber)
	ratelimit := ratelimit.NewBucketWithRate(qps, int64(routineNumber))
	latenciesTable := make([][]int, routineNumber)
	for i := 0; i < routineNumber; i++ {
		go func(index int) {
			startTime := time.Now()
			latencies := []int{}
			for {
				ratelimit.Wait(1)
				start := time.Now()
				ids := CreateContainers(client, 1)
				StartContainers(client, ids)
				end := time.Now()
				latencies = append(latencies, int(end.Sub(start).Nanoseconds()))
				if time.Now().Sub(startTime) >= testPeriod {
					break
				}
			}
			latenciesTable[index] = latencies
			wg.Done()
		}(i)
	}
	wg.Wait()
	allLatencies := []int{}
	for _, latencies := range latenciesTable {
		allLatencies = append(allLatencies, latencies...)
	}
	return allLatencies
}

// DoParalContainerStopBenchmark starts routineNumber of goroutines and let them stop containers, returns latencies of all the stopping calls
// There is a global rate limit on stopping calls per second.
func DoParalContainerStopBenchmark(client *docker.Client, qps float64, routineNumber int) []int {
	ids := GetContainerIds(client)
	idTable := make([][]string, routineNumber)
	for i := 0; i < len(ids); i++ {
		idTable[i%routineNumber] = append(idTable[i%routineNumber], ids[i])
	}
	wg.Add(routineNumber)
	ratelimit := ratelimit.NewBucketWithRate(qps, int64(routineNumber))
	latenciesTable := make([][]int, routineNumber)
	for i := 0; i < routineNumber; i++ {
		go func(index int) {
			latencies := []int{}
			for _, id := range idTable[index] {
				ratelimit.Wait(1)
				start := time.Now()
				StopContainers(client, []string{id})
				RemoveContainers(client, []string{id})
				end := time.Now()
				latencies = append(latencies, int(end.Sub(start).Nanoseconds()))
			}
			latenciesTable[index] = latencies
			wg.Done()
		}(i)
	}
	wg.Wait()
	allLatencies := []int{}
	for _, latencies := range latenciesTable {
		allLatencies = append(allLatencies, latencies...)
	}
	return allLatencies
}

// GetContainerIds returns all the container ids in the system
func GetContainerIds(client *docker.Client) (containerIds []string) {
	containers, err := client.ListContainers(docker.ListContainersOptions{All: true})
	if err != nil {
		panic(fmt.Sprintf("Error list containers: %v", err))
	}
	for _, container := range containers {
		containerIds = append(containerIds, container.ID)
	}
	return containerIds
}

// GetContainerNum returns container number in the system
func GetContainerNum(client *docker.Client, all bool) int {
	containers, err := client.ListContainers(docker.ListContainersOptions{All: all})
	if err != nil {
		panic(fmt.Sprintf("Error list containers: %v", err))
	}
	return len(containers)
}
