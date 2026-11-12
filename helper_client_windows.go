//go:build windows

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/Microsoft/go-winio"
)

var (
	helperClientMu   sync.Mutex
	helperLastOK     time.Time
	helperLastResp   HelperResponse
	helperLastErr    string
	helperCacheTTL   = 2 * time.Second
)

func helperPipePath() string {
	return `\\.\pipe\` + helperPipeName
}

// queryHelper sends a read-only op and returns the response.
func queryHelper(op string, timeout time.Duration) (HelperResponse, error) {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	helperClientMu.Lock()
	if op == "get" && time.Since(helperLastOK) < helperCacheTTL && helperLastResp.OK {
		out := helperLastResp
		helperClientMu.Unlock()
		return out, nil
	}
	helperClientMu.Unlock()

	conn, err := winio.DialPipe(helperPipePath(), &timeout)
	if err != nil {
		helperClientMu.Lock()
		helperLastErr = err.Error()
		helperClientMu.Unlock()
		return HelperResponse{}, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))

	req := HelperRequest{V: helperProtocolVersion, Op: op}
	enc := json.NewEncoder(conn)
	if err := enc.Encode(req); err != nil {
		return HelperResponse{}, err
	}

	var resp HelperResponse
	dec := json.NewDecoder(bufio.NewReader(conn))
	if err := dec.Decode(&resp); err != nil {
		return HelperResponse{}, err
	}
	if resp.V != helperProtocolVersion {
		return HelperResponse{}, fmt.Errorf("helper protocol mismatch: %d", resp.V)
	}
	if !resp.OK && op != "ping" {
		errMsg := resp.Error
		if errMsg == "" {
			errMsg = "helper returned not ok"
		}
		helperClientMu.Lock()
		helperLastErr = errMsg
		helperClientMu.Unlock()
		return resp, fmt.Errorf("%s", errMsg)
	}

	helperClientMu.Lock()
	helperLastOK = time.Now()
	helperLastResp = resp
	helperLastErr = ""
	helperClientMu.Unlock()
	return resp, nil
}

func helperPing() bool {
	resp, err := queryHelper("ping", 1500*time.Millisecond)
	return err == nil && resp.OK && (resp.Pong || resp.OK)
}

func helperGet() (HelperResponse, error) {
	return queryHelper("get", 4*time.Second)
}

// readTempFromHelper is the primary CPU temp source when advanced mode is on.
func readTempFromHelper() externalTempReading {
	if !advancedModeEnabled() {
		return externalTempReading{}
	}
	resp, err := helperGet()
	if err != nil || !resp.CPU.OK {
		return externalTempReading{}
	}
	temp, ok := helperCPUPackageTemp(resp.CPU)
	if !ok {
		return externalTempReading{}
	}
	return externalTempReading{
		TempC: temp, OK: true, Source: "helper",
		LabelRu: "температура из расширенного режима",
		LabelEn: "temperature from advanced mode",
		SensorName: "Tctl/Tdie",
	}
}

func lastHelperError() string {
	helperClientMu.Lock()
	defer helperClientMu.Unlock()
	return helperLastErr
}

func lastHelperOKTime() time.Time {
	helperClientMu.Lock()
	defer helperClientMu.Unlock()
	return helperLastOK
}

func invalidateHelperClientCache() {
	helperClientMu.Lock()
	helperLastOK = time.Time{}
	helperLastResp = HelperResponse{}
	helperClientMu.Unlock()
}
