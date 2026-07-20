/*
Copyright 2026 Serge Logvinov.

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

package recommend

const (
	// CPULowMultiplier defines the multiplier for CPU recommendation requests (20% increase).
	CPULowMultiplier = 1.2
	// CPUHighMultiplier defines the multiplier for CPU recommendation limits (100% increase).
	CPUHighMultiplier = 2.0
	// CPUMinimumRequest defines the minimum CPU request in milli-cores (50m).
	CPUMinimumRequest = 50
	// MemoryLowMultiplier defines the multiplier for memory recommendation requests (20% increase).
	MemoryLowMultiplier = 1.2
	// MemoryHighMultiplier defines the multiplier for memory recommendation limits (100% increase).
	MemoryHighMultiplier = 2.0
	// MemoryMinimumRequest defines the minimum memory request in bytes (64Mi).
	MemoryMinimumRequest = 64 * 1024 * 1024 // 64Mi
)

func roundUpCPULow(milliCores int64) int64 {
	if milliCores <= CPUMinimumRequest {
		return int64(CPUMinimumRequest)
	}

	increment := int64(500) // 500m
	if milliCores < 1000 {
		increment = 100 // 100m for values less than 1 core
	}

	target := int64(float64(milliCores) * CPULowMultiplier)

	return ((target + increment - 1) / increment) * increment
}

func roundUpCPUHigh(milliCores int64) int64 {
	if milliCores <= CPUMinimumRequest {
		return int64(CPUMinimumRequest)
	}

	increment := int64(500) // 500m
	if milliCores < 1000 {
		increment = 100 // 100m for values less than 1 core
	}

	target := int64(float64(milliCores) * CPUHighMultiplier)

	return ((target + increment - 1) / increment) * increment
}

func roundUpMemoryLow(bytes int64) int64 {
	if bytes <= MemoryMinimumRequest {
		return MemoryMinimumRequest
	}

	increment := int64(128 * 1024 * 1024) // 128Mi in bytes
	target := int64(float64(bytes) * MemoryLowMultiplier)

	return ((target + increment - 1) / increment) * increment
}

func roundUpMemoryHigh(bytes int64) int64 {
	if bytes <= MemoryMinimumRequest {
		return MemoryMinimumRequest
	}

	increment := int64(128 * 1024 * 1024) // 128Mi in bytes
	target := int64(float64(bytes) * MemoryHighMultiplier)

	return ((target + increment - 1) / increment) * increment
}
