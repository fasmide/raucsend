package install

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/schollz/progressbar/v3"
	"github.com/xyproto/sheepcounter"
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
	imageSizes, err := u.imageSizes()
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

	go http.Serve(listener, WithProgressbar(imageSizes, http.FileServer(http.Dir("."))))

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
			// remove the progressbar and write
			fmt.Fprint(os.Stdout, "\033[2K\r")
			log.Printf("[%s] %s", tag, scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			log.Printf("[%s ] %s ", tag, err)
		}

	}()
	return writer
}

func WithProgressbar(sizes map[string]int64, h http.Handler) http.Handler {
	var bar *progressbar.ProgressBar
	uri := ""

	logFn := func(rw http.ResponseWriter, r *http.Request) {
		// if uri changes, invent a new progressbar
		if uri != r.RequestURI {
			uri = r.RequestURI
			bar = progressbar.NewOptions(int(sizes[uri[1:]]),
				progressbar.OptionShowBytes(true),
				progressbar.OptionShowCount(),
				progressbar.OptionSetDescription(uri),
			)
		}

		sc := sheepcounter.New(rw)

		h.ServeHTTP(sc, r)

		bar.Add64(sc.Counter())
	}
	return http.HandlerFunc(logFn)
}

func (u *Uploader) imageSizes() (map[string]int64, error) {
	imageSizes := make(map[string]int64)
	for _, v := range u.Images {
		info, err := os.Stat(v)
		if err != nil {
			return nil, err
		}
		imageSizes[v] = info.Size()
	}
	return imageSizes, nil
}
