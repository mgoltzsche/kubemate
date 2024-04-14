package main

import (
	"context"
	"errors"
	"os"

	"github.com/k3s-io/k3s/pkg/cli/agent"
	"github.com/k3s-io/k3s/pkg/cli/cert"
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/cli/completion"
	"github.com/k3s-io/k3s/pkg/cli/crictl"
	"github.com/k3s-io/k3s/pkg/cli/etcdsnapshot"
	"github.com/k3s-io/k3s/pkg/cli/kubectl"
	"github.com/k3s-io/k3s/pkg/cli/secretsencrypt"
	"github.com/k3s-io/k3s/pkg/cli/server"
	"github.com/k3s-io/k3s/pkg/configfilearg"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	addcmds "github.com/mgoltzsche/kubemate/pkg/cli/cmds"
)

// Copied from https://github.com/k3s-io/k3s/blob/v1.29.3%2Bk3s1/cmd/server/main.go and added `connect` command.

func main() {
	app := cmds.NewApp()
	app.Commands = []cli.Command{
		addcmds.NewConnectCommand(addcmds.RunConnectServer),
		cmds.NewServerCommand(server.Run),
		cmds.NewAgentCommand(agent.Run),
		cmds.NewKubectlCommand(kubectl.Run),
		cmds.NewCRICTL(crictl.Run),
		cmds.NewEtcdSnapshotCommands(
			etcdsnapshot.Delete,
			etcdsnapshot.List,
			etcdsnapshot.Prune,
			etcdsnapshot.Save,
		),
		cmds.NewSecretsEncryptCommands(
			secretsencrypt.Status,
			secretsencrypt.Enable,
			secretsencrypt.Disable,
			secretsencrypt.Prepare,
			secretsencrypt.Rotate,
			secretsencrypt.Reencrypt,
			secretsencrypt.RotateKeys,
		),
		cmds.NewCertCommands(
			cert.Rotate,
			cert.RotateCA,
		),
		cmds.NewCompletionCommand(completion.Run),
	}

	if err := app.Run(configfilearg.MustParse(os.Args)); err != nil && !errors.Is(err, context.Canceled) {
		logrus.Fatal(err)
	}
}
