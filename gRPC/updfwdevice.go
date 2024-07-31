package gRPC

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	sh "github.com/roxxxiey/ProtoForStend/go"
	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc"
	"io"
	"log"
	"os"
	"strings"
	"time"
)

type TFTPviaSSH struct {
	sh.UnimplementedFirmwareDeviceServer
}

func RegisterSSHClient(gRPCServer *grpc.Server) {
	sh.RegisterFirmwareDeviceServer(gRPCServer, &TFTPviaSSH{})
}

const (
	Type = "TFTPviaSSH"
)

var stderrBuf bytes.Buffer

var (
	// ErrWithTimeReadFile this error works when there are problems with downloading a file
	ErrWithTimeReadFile   = errors.New("problems with downloading a file: i/o timeout")
	ErrWithCRC16          = errors.New("invalid CRC16 answer (is not equal 0x0)")
	ErrWithReadyToUpgrade = errors.New("device  is not ready to upgrade")
	ErrRetries            = errors.New("update firmware failed after multiple retries")
)

// UPDFWType has not been implemented yet
func (t *TFTPviaSSH) UPDFWType(ctx context.Context, request *sh.UPDFWTypeRequest) (*sh.UPDFWTypeResponse, error) {
	log.Println("Call ChangeType")
	return nil, nil
}

func (t *TFTPviaSSH) UpdateFirmware(ctx context.Context, request *sh.UpdateFirmwareRequest) (*sh.UpdateFirmwareResponse, error) {
	log.Println("Call Udate Firmware")

	/*safe := flag.String("safe", "", "Safe file name")
	flag.Parse()*/

	settings := request.GetSettings()
	ip := settings[0].GetValue()
	user := settings[1].GetValue()
	password := settings[2].GetValue()
	pathToFile := settings[3].GetValue()
	ipTftpSever := settings[4].GetValue()

	maxRetries := 3
	retryDelay := 5

	for retries := 0; retries < maxRetries; retries++ {
		clientConfig := &ssh.ClientConfig{
			User: user,
			Auth: []ssh.AuthMethod{ssh.Password(password)},
		}
		clientConfig.HostKeyCallback = ssh.InsecureIgnoreHostKey()

		client, err := ssh.Dial("tcp", fmt.Sprintf("%s:22", ip), clientConfig)
		if err != nil {
			log.Printf("SSH Dial failed: %v. Retrying (%d/%d)...", err, retries+1, maxRetries)
			time.Sleep(time.Duration(retryDelay))
			continue
		}
		defer client.Close()

		session, err := client.NewSession()
		if err != nil {
			log.Printf("SSH NewSession failed: %v. Retrying (%d/%d)...", err, retries+1, maxRetries)
			time.Sleep(time.Duration(retryDelay))
			continue
		}
		defer session.Close()

		log.Println("Session created")

		stdin, err := session.StdinPipe()
		if err != nil {
			log.Printf("Unable to setup STDIN: %v. Retrying (%d/%d)...", err, retries+1, maxRetries)
			time.Sleep(time.Duration(retryDelay))
			continue
		}
		defer stdin.Close()

		stderr, err := session.StderrPipe()
		if err != nil {
			log.Printf("Unable to setup stderr for session: %v. Retrying (%d/%d)...", err, retries+1, maxRetries)
			time.Sleep(time.Duration(retryDelay))
			continue
		}
		go io.Copy(os.Stderr, stderr)

		var stdoutBuf bytes.Buffer
		session.Stdout = io.MultiWriter(os.Stdout, &stdoutBuf)

		if err = session.Shell(); err != nil {
			log.Printf("Unable to setup shell: %v. Retrying (%d/%d)...", err, retries+1, maxRetries)
			time.Sleep(time.Duration(retryDelay))
			continue
		}

		getter := flag.Lookup("safe")
		com := getter.Value.String()

		if com != "" {
			safeCommand := fmt.Sprintf("util tftp " + ipTftpSever + " put config safeConf.json")
			if err = t.sendCommand(stdin, safeCommand); err != nil {
				log.Printf("Failed to send first command: %v. Retrying (%d/%d)...", err, retries+1, maxRetries)
				time.Sleep(time.Duration(retryDelay))
				continue
			}
		}

		commands := []string{
			fmt.Sprintf("util tftp %s get image %s", ipTftpSever, pathToFile),
			"device upgrade image",
			"reboot",
		}

		if err = t.sendCommand(stdin, commands[0]); err != nil {
			log.Printf("Failed to send first command: %v. Retrying (%d/%d)...", err, retries+1, maxRetries)
			time.Sleep(time.Duration(retryDelay))
			continue
		}

		if err = t.monitorAnswers("CRC16 = 0x0", 5, 5*time.Second, &stdoutBuf); err != nil {
			log.Printf("Failed to monitor CRC16: %v. Retrying (%d/%d)...", err, retries+1, maxRetries)
			stdoutBuf.Reset()
			time.Sleep(time.Duration(retryDelay))
			continue
		}

		time.Sleep(10 * time.Second)

		stdoutBuf.Reset()
		if err = t.sendCommand(stdin, commands[1]); err != nil {
			log.Printf("Failed to send second command: %v. Retrying (%d/%d)...", err, retries+1, maxRetries)
			time.Sleep(time.Duration(retryDelay))
			continue
		}
		time.Sleep(1 * time.Second)

		if err = t.monitorAnswers("OK: device is ready for upgrade", 5, 5*time.Second, &stdoutBuf); err != nil {
			log.Printf("Failed to monitor device readiness: %v. Retrying (%d/%d)...", err, retries+1, maxRetries)
			stdoutBuf.Reset()
			time.Sleep(time.Duration(retryDelay))
			continue
		}

		stdoutBuf.Reset()
		if err = t.sendCommand(stdin, commands[2]); err != nil {
			log.Printf("Failed to send third command: %v. Retrying (%d/%d)...", err, retries+1, maxRetries)
			time.Sleep(time.Duration(retryDelay))
			continue
		}
		time.Sleep(1 * time.Second)

		log.Printf("Commands executed successfully")

		if err = session.Wait(); err != nil {
			log.Printf("Failed to wait for session: %v. Retrying (%d/%d)...", err, retries+1, maxRetries)
			time.Sleep(time.Duration(retryDelay))
			continue
		}

		return &sh.UpdateFirmwareResponse{
			Status: "Command executed successfully",
		}, nil
	}

	return nil, ErrRetries
}

func (t *TFTPviaSSH) monitorAnswers(prefix string, attempts int, delay time.Duration, stdoutBuf *bytes.Buffer) error {
	tick := time.NewTicker(delay)
	defer tick.Stop()
	count := 0
	var bufOut string

	for range tick.C {
		bufOut = stdoutBuf.String()
		count++
		if !strings.Contains(bufOut, prefix) && count != attempts {
			log.Println("Выполняю проверку !strings.Contains(bufOut, prefix) && count != attempts ")
			log.Println("It is stdoutBUFFER: ", bufOut)
			continue
		}
		if !strings.Contains(bufOut, prefix) && count == attempts {
			log.Println("Выполняю проверку !strings.Contains(bufOut, prefix) && count == attempts ")
			log.Println("It is stdoutBUFFER: ", bufOut)
			log.Println(stderrBuf.String())
			if prefix == "OK: device is ready for upgrade" {
				return ErrWithReadyToUpgrade
			}
			if t.checkCRC(bufOut) {
				return ErrWithCRC16
			}
			return ErrWithTimeReadFile
		}

		if strings.Contains(bufOut, prefix) {
			log.Println("Выполняю проверку strings.Contains(bufOut, prefix) ")
			break
		}
	}
	return nil
}

func (t TFTPviaSSH) sendCommand(in io.WriteCloser, command string) error {
	if _, err := in.Write([]byte(command + "\r\n")); err != nil {
		return err
	}
	return nil
}

func (t TFTPviaSSH) checkCRC(bufOut string) bool {
	if strings.Contains(bufOut, "CRC = ") {
		return true
	}
	return false
}
