package util

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"sync"
	"time"
	"unsafe"

	"github.com/btcsuite/btcutil/base58"
)

func Truncate(data []byte, limit int) []byte {
	l := len(data)
	if l > limit {
		l = limit
	}
	return data[:l]
}

func FromHexAddress(address string) (string, error) {
	data, err := hex.DecodeString(address)
	if err != nil {
		return "", fmt.Errorf("invalid address(%s)", address)
	}

	sum1 := sha256.Sum256(data)
	checksum := sha256.Sum256(sum1[:])
	data = append(data, checksum[0:4]...)
	buffer := base58.Encode(data)
	return buffer, nil
}

func ToHexAddress(address string) (string, error) {
	buffer := base58.Decode(address)
	if len(buffer) != 24 {
		return "", fmt.Errorf("invalid base58 address(%s)", address)
	}
	payload := buffer[0:20]
	checksum := buffer[20:]
	sum1 := sha256.Sum256(payload)
	newChecksum := sha256.Sum256(sum1[:])

	if !bytes.Equal(checksum, newChecksum[0:4]) {
		return "", fmt.Errorf("base58 address(%s) checksum(%s) mismatched %s", address, string(checksum), string(newChecksum[:]))
	}

	return hex.EncodeToString(payload), nil
}

func HttpGet(host string, path string, query url.Values) ([]byte, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("http://%s%s", host, path), nil)
	if err != nil {
		return nil, err
	}
	req.URL.RawQuery = query.Encode()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		resp.Body.Close()
		return nil, err
	}

	if resp.StatusCode != 200 {
		resp.Body.Close()
		return nil, fmt.Errorf("http get %s failed %s", req.URL.String(), body)
	}
	resp.Body.Close()
	return body, nil
}

func Str2bytes(s string) []byte {
	x := (*[2]uintptr)(unsafe.Pointer(&s))
	h := [3]uintptr{x[0], x[1], x[1]}
	return *(*[]byte)(unsafe.Pointer(&h))
}

func Bytes2str(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

func HasBit(n byte, pos uint) bool {
	pos = 7 - pos
	val := n & (1 << pos)
	return (val > 0)
}

func RandomIt(data []byte) {
	_, _ = rand.Read(data)
}

func RandomUint32() uint32 {
	data := make([]byte, 4)
	RandomIt(data)
	return *(*uint32)(unsafe.Pointer(&data[0]))
}

func GoWithWaitGroup(wg *sync.WaitGroup, f func()) {
	wg.Add(1)
	go func() {
		f()
		wg.Done()
	}()
}

func GoWithPanicRecover(f func()) <-chan struct{} {
	panicChannel := make(chan struct{}, 1)

	go func() {
		defer func() {
			if err := recover(); err != nil {
				panicChannel <- struct{}{}
			}
		}()
		f()
	}()

	return panicChannel
}

const (
	BNRTC_CHILD_MODE_KEY        = "BNRTC_RUN_MODE"
	BNRTC_CHILD_MODE_VALUE      = "child"
	BNRTC_CHILD_MODE_ENV_STRING = BNRTC_CHILD_MODE_KEY + "=" + BNRTC_CHILD_MODE_VALUE

	BNRTC_HAS_EXCEPTION_KEY    = "BNRTC_HAS_EXCEPTION"
	BNRTC_HAS_EXCEPTION_VALUE  = "true"
	BNRTC_HAS_EXCEPTION_STRING = BNRTC_HAS_EXCEPTION_KEY + "=" + BNRTC_HAS_EXCEPTION_VALUE
)

func IsChildProcess() bool {
	return os.Getenv(BNRTC_CHILD_MODE_KEY) == BNRTC_CHILD_MODE_VALUE
}

func ForkRunLoop() error {
	return ForkRunLoopWithAbortFunc(nil)
}

func ForkRunLoopWithAbortFunc(f func(error) bool) error {
	if IsChildProcess() {
		return errors.New("already in child process")
	}

	hasException := false
	args := os.Args[1:]
	errCount := 0
	lastErrtime := time.Now()
	for {
		fmt.Println("forking...")
		cmd := exec.Command(os.Args[0], args...)
		cmd.Env = os.Environ()
		cmd.Env = append(cmd.Env, BNRTC_CHILD_MODE_ENV_STRING)
		if hasException {
			cmd.Env = append(cmd.Env, BNRTC_HAS_EXCEPTION_STRING)
		}
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err != nil {
			fmt.Println("fork run failed:", err)
			if f != nil && f(err) {
				return err
			}
			if time.Since(lastErrtime) > time.Second*30 {
				errCount = 0
			}
			errCount++
			lastErrtime = time.Now()
			if errCount > 5 {
				return err
			}

			hasException = true
		}
	}
}
