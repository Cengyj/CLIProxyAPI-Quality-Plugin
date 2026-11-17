//go:build !windows

package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/cli-proxy-api/model-quality-login-runtime/internal/jshost"
)

func TestMain(m *testing.M) {
	if handled, code := jshost.WorkerMain(os.Args[1:], os.Stdin, os.Stdout); handled {
		os.Exit(code)
	}
	if len(os.Args) == 4 && os.Args[1] == "--owner-supervisor-fixture" {
		os.Exit(ownerSupervisorFixture(os.Args[2], os.Args[3]))
	}
	if len(os.Args) == 4 && os.Args[1] == "--owner-runtime-fixture" {
		os.Exit(ownerRuntimeFixture(os.Args[2], os.Args[3]))
	}
	os.Exit(m.Run())
}

func ownerSupervisorFixture(inputPath, readyPath string) int {
	reader, writer, err := os.Pipe()
	if err != nil {
		return 1
	}
	defer writer.Close()
	command := exec.Command(os.Args[0], "--owner-runtime-fixture", inputPath, readyPath)
	command.ExtraFiles = []*os.File{reader}
	command.Env = append(os.Environ(), "MQ_LOGIN_OWNER_FD=3")
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err = command.Start(); err != nil {
		return 1
	}
	_ = reader.Close()
	_, _ = io.Copy(io.Discard, os.Stdin)
	_ = writer.Close()
	_ = command.Wait()
	return 0
}

func ownerRuntimeFixture(inputPath, readyPath string) int {
	ctx, closeOwner, err := ownerContext(context.Background(), os.Getenv("MQ_LOGIN_OWNER_FD"))
	if err != nil {
		return 1
	}
	defer closeOwner()
	command := exec.CommandContext(ctx, os.Args[0], "--auth-js-worker", "sentinel", inputPath)
	input, err := command.StdinPipe()
	if err != nil {
		return 1
	}
	defer input.Close()
	output, err := command.StdoutPipe()
	if err != nil || command.Start() != nil {
		return 1
	}
	// Complete a real Sobek call before starting the unbounded second call.
	_, _ = io.WriteString(input, "{\"type\":\"call\",\"method\":\"token\",\"flow\":\"fixture\"}\n")
	if !bufio.NewScanner(output).Scan() {
		_ = command.Process.Kill()
		_ = command.Wait()
		return 1
	}
	_, _ = io.WriteString(input, "{\"type\":\"call\",\"method\":\"token\",\"flow\":\"fixture\"}\n")
	ready := fmt.Sprintf("%d %d", os.Getpid(), command.Process.Pid)
	if os.WriteFile(readyPath, []byte(ready), 0o600) != nil {
		_ = command.Process.Kill()
		_ = command.Wait()
		return 1
	}
	<-ctx.Done()
	_ = input.Close()
	_ = command.Wait()
	return 0
}

func TestOwnerDeathStopsNativeWorkerAndSobekChild(t *testing.T) {
	private := t.TempDir()
	inputPath, readyPath := filepath.Join(private, "input.json"), filepath.Join(private, "ready")
	source := `{"url":"https://example.test/","html":"<html></html>","loaderUrl":"https://example.test/sdk.js","loaderSource":"let calls=0;globalThis.SentinelSDK={token:()=>{if(calls++===0)return 'ready';while(true){}}};"}`
	if err := os.WriteFile(inputPath, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	supervisor := exec.Command(os.Args[0], "--owner-supervisor-fixture", inputPath, readyPath)
	input, err := supervisor.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	if err = supervisor.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = supervisor.Process.Kill(); _ = supervisor.Wait() })
	deadline := time.Now().Add(5 * time.Second)
	var pids []int
	for len(pids) != 2 {
		data, _ := os.ReadFile(readyPath)
		pids = nil
		for _, value := range strings.Fields(string(data)) {
			pid, err := strconv.Atoi(value)
			if err != nil || pid <= 1 {
				t.Fatal("invalid owner fixture PID")
			}
			pids = append(pids, pid)
		}
		if len(pids) == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("native worker and Sobek child did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Cleanup(func() { _ = syscall.Kill(-pids[0], syscall.SIGKILL) })
	for _, pid := range pids {
		if err = syscall.Kill(pid, 0); err != nil {
			t.Fatal("owned process exited before supervisor death")
		}
	}
	if err = supervisor.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = supervisor.Wait()
	deadline = time.Now().Add(3 * time.Second)
	for _, pid := range pids {
		for !errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
			if time.Now().After(deadline) {
				t.Fatalf("owned process %d survived its launcher", pid)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestOwnerContextWithoutPipeRetainsOrdinaryInputSemantics(t *testing.T) {
	ctx, closeOwner, err := ownerContext(t.Context(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer closeOwner()
	if ctx.Err() != nil {
		t.Fatal("optional owner pipe canceled an ordinary invocation")
	}
	if _, _, err = ownerContext(t.Context(), "0"); err == nil {
		t.Fatal("stdin was accepted as an ownership channel")
	}
}
