package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/docker/docker/pkg/reexec"
	"github.com/k3s-io/k3s/pkg/cli/agent"
	"github.com/k3s-io/k3s/pkg/cli/cert"
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/cli/completion"
	"github.com/k3s-io/k3s/pkg/cli/crictl"
	"github.com/k3s-io/k3s/pkg/cli/ctr"
	"github.com/k3s-io/k3s/pkg/cli/etcdsnapshot"
	"github.com/k3s-io/k3s/pkg/cli/kubectl"
	"github.com/k3s-io/k3s/pkg/cli/secretsencrypt"
	"github.com/k3s-io/k3s/pkg/cli/server"
	"github.com/k3s-io/k3s/pkg/cli/token"
	"github.com/k3s-io/k3s/pkg/configfilearg"
	"github.com/k3s-io/k3s/pkg/containerd"
	ctr2 "github.com/k3s-io/k3s/pkg/ctr"
	kubectl2 "github.com/k3s-io/k3s/pkg/kubectl"
	crictl2 "github.com/kubernetes-sigs/cri-tools/cmd/crictl"
	addcmds "github.com/mgoltzsche/kubemate/pkg/cli/cmds"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

// Copied from https://github.com/k3s-io/k3s/blob/v1.26.3%2Bk3s1/cmd/server/main.go and added `connect` command.

func init() {
	reexec.Register("containerd", containerd.Main)
	reexec.Register("kubectl", kubectl2.Main)
	reexec.Register("crictl", crictl2.Main)
	reexec.Register("ctr", ctr2.Main)
}

func main() {
	cmd := os.Args[0]
	os.Args[0] = filepath.Base(os.Args[0])
	if reexec.Init() {
		return
	}
	os.Args[0] = cmd

	app := cmds.NewApp()
	app.Commands = []cli.Command{
		addcmds.NewConnectCommand(addcmds.RunConnectServer),
		cmds.NewServerCommand(server.Run),
		cmds.NewAgentCommand(agent.Run),
		cmds.NewKubectlCommand(kubectl.Run),
		cmds.NewCRICTL(crictl.Run),
		cmds.NewCtrCommand(ctr.Run),
		cmds.NewTokenCommands(
			token.Create,
			token.Delete,
			token.Generate,
			token.List,
		),
		cmds.NewEtcdSnapshotCommands(
			etcdsnapshot.Run,
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
		),
		cmds.NewCertCommand(
			cmds.NewCertSubcommands(
				cert.Rotate,
				cert.RotateCA,
			),
		),
		cmds.NewCompletionCommand(completion.Run),
	}

	if err := app.Run(configfilearg.MustParse(os.Args)); err != nil && !errors.Is(err, context.Canceled) {
		logrus.Fatal(err)
	}
}
