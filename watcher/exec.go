package cli

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// GoRun  is an implementation of the bin execution
func (p *Project) goRun(channel chan bool, runner chan bool, wr *sync.WaitGroup) error {

	var build *exec.Cmd
	if len(p.Params) != 0 {
		build = exec.Command(filepath.Join(os.Getenv("GOBIN"), filepath.Base(p.path)), p.Params...)
	} else {
		build = exec.Command(filepath.Join(os.Getenv("GOBIN"), filepath.Base(p.path)))
	}
	build.Dir = p.base
	defer func() {
		if err := build.Process.Kill(); err != nil {
			p.Buffer.StdLog = append(p.Buffer.StdLog, BufferOut{Time: time.Now(), Text: "Failed to stop: " + err.Error()})
			p.Fatal("Failed to stop:", err)
		}
		p.Buffer.StdLog = append(p.Buffer.StdLog, BufferOut{Time: time.Now(), Text: "Ended"})
		log.Println(p.pname(p.Name, 2), ":", p.Red.Regular("Ended"))
		go p.sync()
		wr.Done()
	}()

	stdout, err := build.StdoutPipe()
	stderr, err := build.StderrPipe()

	if err != nil {
		log.Println(p.Red.Bold(err.Error()))
		return err
	}
	if err := build.Start(); err != nil {
		log.Println(p.Red.Bold(err.Error()))
		return err
	}
	close(runner)

	execOutput, execError := bufio.NewScanner(stdout), bufio.NewScanner(stderr)
	stopOutput, stopError := make(chan bool, 1), make(chan bool, 1)

	scanner := func(stop chan bool, output *bufio.Scanner, isError bool) {
		for output.Scan() {
			select {
			default:
				if isError {
					p.Buffer.StdErr = append(p.Buffer.StdErr, BufferOut{Time: time.Now(), Text: output.Text()})
				} else {
					p.Buffer.StdOut = append(p.Buffer.StdOut, BufferOut{Time: time.Now(), Text: output.Text()})
				}
				go p.sync()
				if p.Cli.Streams {
					log.Println(p.pname(p.Name, 3), ":", p.Blue.Regular(output.Text()))
				}
				if p.File.Streams {
					path := filepath.Join(p.base, p.parent.Resources.Output)
					f := p.Create(path)
					t := time.Now()
					if _, err := f.WriteString(t.Format("2006-01-02 15:04:05") + " : " + output.Text() + "\r\n"); err != nil {
						p.Fatal("", err)
					}
				}
			}
		}
		close(stop)
	}
	p.Buffer.StdLog = append(p.Buffer.StdLog, BufferOut{Time: time.Now(), Text: "Started"})
	go scanner(stopOutput, execOutput, false)
	go scanner(stopError, execError, true)

	for {
		select {
		case <-channel:
			return nil
		case <-stopOutput:
			return nil
		case <-stopError:
			return nil
		}
	}
}

// GoBuild is an implementation of the "go build"
func (p *Project) goBuild() (string, error) {
	defer func() {
		p.sync()
	}()
	var out bytes.Buffer
	var stderr bytes.Buffer
	build := exec.Command("go", "build")
	build.Dir = p.base
	build.Stdout = &out
	build.Stderr = &stderr
	if err := build.Run(); err != nil {
		p.Buffer.StdErr = append(p.Buffer.StdErr, BufferOut{Time: time.Now(), Text: err.Error()})
		return stderr.String(), err
	}
	return "", nil
}

// GoInstall is an implementation of the "go install"
func (p *Project) goInstall() (string, error) {
	defer func() {
		p.sync()
	}()
	var out bytes.Buffer
	var stderr bytes.Buffer
	err := os.Setenv("GOBIN", filepath.Join(os.Getenv("GOPATH"), "bin"))
	if err != nil {
		p.Buffer.StdErr = append(p.Buffer.StdErr, BufferOut{Time: time.Now(), Text: err.Error()})
		return "", err
	}
	build := exec.Command("go", "install")
	build.Dir = p.base
	build.Stdout = &out
	build.Stderr = &stderr
	if err := build.Run(); err != nil {
		p.Buffer.StdErr = append(p.Buffer.StdErr, BufferOut{Time: time.Now(), Text: err.Error()})
		return stderr.String(), err
	}
	return "", nil
}

// GoFmt is an implementation of the gofmt
func (p *Project) goFmt(path string) (io.Writer, error) {
	defer func() {
		p.sync()
	}()
	var out bytes.Buffer
	build := exec.Command("gofmt", "-s", "-w", "-e", path)
	build.Dir = p.base
	build.Stdout = &out
	build.Stderr = &out
	if err := build.Run(); err != nil {
		fmt.Print("append")
		p.Buffer.StdErr = append(p.Buffer.StdErr, BufferOut{Time: time.Now(), Text: err.Error()})
		return build.Stderr, err
	}
	return nil, nil
}

// GoTest is an implementation of the go test
func (p *Project) goTest(path string) (io.Writer, error) {
	defer func() {
		p.sync()
	}()
	var out bytes.Buffer
	build := exec.Command("go", "test")
	build.Dir = path
	build.Stdout = &out
	build.Stderr = &out
	if err := build.Run(); err != nil {
		p.Buffer.StdErr = append(p.Buffer.StdErr, BufferOut{Time: time.Now(), Text: err.Error()})
		return build.Stdout, err
	}
	return nil, nil
}

// GoGenerate is an implementation of the go test
func (p *Project) goGenerate(path string) (io.Writer, error) {
	defer func() {
		p.sync()
	}()
	var out bytes.Buffer
	build := exec.Command("go", "generate")
	build.Dir = path
	build.Stdout = &out
	build.Stderr = &out
	if err := build.Run(); err != nil {
		p.Buffer.StdErr = append(p.Buffer.StdErr, BufferOut{Time: time.Now(), Text: err.Error()})
		return build.Stdout, err
	}
	return nil, nil
}

// Cmds exec a list of defined commands
func (p *Project) cmds(cmds []string) (errors []error) {
	for _, cmd := range cmds {
		cmd := strings.Replace(strings.Replace(cmd, "'", "", -1), "\"", "", -1)
		c := strings.Split(cmd, " ")
		build := exec.Command(c[0], c[1:]...)
		build.Dir = p.base
		if err := build.Run(); err != nil {
			p.Buffer.StdErr = append(p.Buffer.StdErr, BufferOut{Time: time.Now(), Text: err.Error()})
			errors = append(errors, err)
			return errors
		}
	}
	return nil
}

// Sync datas with the web server
func (p *Project) sync() {
	go func() {
		p.parent.Sync <- "sync"
	}()
}
