package cmd

import (
	"os"

	"github.com/fasmide/raucsend/install"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
)

var user string
var pass string

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
		job := install.Uploader{
			Target: args[0],
			Images: args[1:],
			SSHConfig: &ssh.ClientConfig{
				User: user,
				Auth: []ssh.AuthMethod{
					ssh.Password(pass),
				},
				HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			},
		}
		return job.Run()
	},
}

func init() {
	rootCmd.AddCommand(installCmd)
	installCmd.Flags().StringVarP(&user, "user", "l", "root", "ssh user")
	installCmd.Flags().StringVarP(&pass, "pass", "p", "root", "ssh password")
}
