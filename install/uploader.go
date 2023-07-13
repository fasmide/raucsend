package install

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"golang.org/x/crypto/ssh"
)

type Uploader struct {
	Target    string
	SSHConfig *ssh.ClientConfig
	Images    []string

	Reboot bool

	location string
	sshConn  *ssh.Client
}

// Run will initiate the webserver and setup forward ssh tunnel to target
func (u *Uploader) Run() error {
	err := u.checkImages()
	if err != nil {
		return fmt.Errorf("error checking images: %w", err)
	}

	u.sshConn, err = ssh.Dial("tcp", u.Target, u.SSHConfig)
	if err != nil {
		return fmt.Errorf("unable to dial ssh host %s: %w", u.Target, err)
	}
	defer u.sshConn.Close()

	log.Printf("connection to ssh://%s succeded", u.Target)

	listener, err := u.sshConn.Listen("tcp", "localhost:0")
	if err != nil {
		return fmt.Errorf("unable to forward embedded webserver: %w", err)
	}

	u.location = listener.Addr().String()

	// TODO: dont use a fileserver, but a custom handler that will serve the images
	// and nothing more - maybe even output a progressbar as we will know
	// how far along the transmit is.
	// but this is quick and dirty
	go http.Serve(listener, http.FileServer(http.Dir(".")))

	log.Printf("fileserver listening on remote: %s", u.location)

	for _, v := range u.Images {
		session, err := u.sshConn.NewSession()
		if err != nil {
			return fmt.Errorf("unable to open session: %w", err)
		}
		defer session.Close()

		session.Stdout = SpecialOutput("\x1b[36m-->\033[0m")
		session.Stderr = SpecialOutput("\x1b[31mERR\033[0m")

		cmd := fmt.Sprintf("rauc install http://%s/%s", u.location, v)

		log.Printf("[\x1b[32m<--\033[0m] %s", cmd)

		err = session.Run(cmd)
		if err != nil {
			return fmt.Errorf("unable to run remote command: %w", err)
		}

		session.Close()
	}

	if u.Reboot {
		session, err := u.sshConn.NewSession()
		if err != nil {
			return fmt.Errorf("unable to open session: %w", err)
		}
		defer session.Close()

		session.Stdout = SpecialOutput("\x1b[36m-->\033[0m")
		session.Stderr = SpecialOutput("\x1b[31mERR\033[0m")

		log.Printf("[\x1b[32m<--\033[0m] %s", "reboot")

		err = session.Run("reboot")
		_, ok := err.(*ssh.ExitMissingError)
		if ok {
			log.Printf("connection lost, device rebooting...")
			return nil
		}
		if err != nil {
			return fmt.Errorf("unable to run remote command: %w", err)
		}

		session.Close()
	}

	return nil
}

func SpecialOutput(tag string) io.Writer {
	reader, writer := io.Pipe()
	go func() {
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			log.Printf("[%s] %s", tag, scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			log.Printf("[%s ] %s ", tag, err)
		}

	}()
	return writer
}

func (u *Uploader) checkImages() error {
	for _, v := range u.Images {
		_, err := os.Stat(v)
		if err != nil {
			return err
		}
	}
	return nil
}
