package cmd

import (
	"net"
	"os"

	"github.com/fasmide/raucsend/install"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
)

var user string
var pass string
var reboot bool

// installCmd represents the install command
var installCmd = &cobra.Command{
	Use:   "install <target> <image1> <image2> ...",
	Short: "runs rauc install on target, forwards embedded webserver for serving images",
	Args:  cobra.MinimumNArgs(2),
	PreRun: func(cmd *cobra.Command, args []string) {
		envPass := os.Getenv("RAUCSEND_SSH_PASS")
		if envPass != "" {
			pass = envPass
		}
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		host, port, err := net.SplitHostPort(args[0])
		if err != nil {
			// default to args[0] - whatever that may be
			host = args[0]
			port = "22" // port 22
		}

		job := install.Uploader{
			Target: net.JoinHostPort(host, port),
			Images: args[1:],
			SSHConfig: &ssh.ClientConfig{
				User: user,
				Auth: []ssh.AuthMethod{
					ssh.Password(pass),
				},
				HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			},
			Reboot: reboot,
		}
		return job.Run()
	},
}

func init() {
	rootCmd.AddCommand(installCmd)
	installCmd.Flags().StringVarP(&user, "user", "l", "root", "ssh user")
	installCmd.Flags().StringVarP(&pass, "pass", "p", "root", "ssh password")
	installCmd.Flags().BoolVarP(&reboot, "reboot", "r", false, "reboot device on success")
}
