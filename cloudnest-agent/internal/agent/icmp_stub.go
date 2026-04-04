//go:build !linux

package agent

// runICMP is a stub for non-Linux builds.
func runICMP(target string) (latency float64, success bool) {
	return 0, false
}
