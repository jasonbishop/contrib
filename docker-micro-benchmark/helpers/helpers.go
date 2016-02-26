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
	"math"
	"sort"
	"strings"
	"time"
)

var rates = []float64{0.5, 0.75, 0.95, 0.99}

var last = time.Now()

func logTime(label string) {
	now := time.Now()
	fmt.Printf("%02d:%02d:%02d:%02d\t%s\n", last.Day(), last.Hour(), last.Minute(), last.Second(), label)
	fmt.Printf("%02d:%02d:%02d:%02d\t%s\n", now.Day(), now.Hour(), now.Minute(), now.Second(), label)
	last = now
}

// LogTitle prints an empty line and the title of the benchmark
func LogTitle(title string) {
	fmt.Println()
	fmt.Println(title)
}

// LogEVar prints all the environemnt variables
func LogEVar(vars map[string]interface{}) {
	for k, v := range vars {
		fmt.Printf("%s=%v ", k, v)
	}
	fmt.Println()
}

// LogLabels prints the labels of the result table
func LogLabels(labels ...string) {
	fmt.Printf("time\t%%50\t%%75\t%%95\t%%99\t%s\n", strings.Join(labels, "\t"))
}

// LogResult prints the item of the result table
func LogResult(latencies []int, variables ...string) {
	average := func(latencies []int) int {
		if len(latencies) <= 0 {
			return 0
		}
		total := 0
		for _, l := range latencies {
			total += l
		}
		return total / len(latencies)
	}

	sort.Ints(latencies)
	var avgs [4]float64
	for i, rate := range rates {
		n := int(math.Ceil((1 - rate) * float64(len(latencies))))
		avgs[i] = float64(average(latencies[len(latencies)-n:])) / 1000000
	}
	logTime(fmt.Sprintf("%.2f\t%.2f\t%.2f\t%.2f\t%s", avgs[0], avgs[1], avgs[2], avgs[3], strings.Join(variables, "\t")))
}

// Itoas converts int numbers to a slice of string
func Itoas(nums ...int) []string {
	r := []string{}
	for _, n := range nums {
		r = append(r, fmt.Sprintf("%d", n))
	}
	return r
}

// Ftoas converts float64 numbers to a slice of string
func Ftoas(nums ...float64) []string {
	r := []string{}
	for _, n := range nums {
		r = append(r, fmt.Sprintf("%0.4f", n))
	}
	return r
}
