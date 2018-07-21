package main

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/cmd/ctr/commands/content"
	"github.com/containerd/containerd/contrib/apparmor"
	"github.com/containerd/containerd/contrib/seccomp"
	"github.com/containerd/containerd/defaults"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/urfave/cli"
)

var runCommand = cli.Command{
	Name:  "run",
	Usage: "run a container",
	Action: func(clix *cli.Context) error {
		var config Config
		if _, err := toml.DecodeFile(clix.Args().First(), &config); err != nil {
			return err
		}
		ctx := namespaces.WithNamespace(context.Background(), clix.GlobalString("namespace"))
		client, err := containerd.New(
			defaults.DefaultAddress,
			containerd.WithDefaultRuntime("io.containerd.runc.v1"),
		)
		if err != nil {
			return err
		}
		defer client.Close()
		image, err := content.Fetch(ctx, client, config.Image, clix)
		if err != nil {
			return err
		}
		fmt.Printf("unpacking image into %s\n", containerd.DefaultSnapshotter)
		if err := image.Unpack(ctx, containerd.DefaultSnapshotter); err != nil {
			return err
		}
		opts := []oci.SpecOpts{
			oci.WithImageConfig(image),
			oci.WithHostLocaltime,
			oci.WithNoNewPrivileges,
			apparmor.WithDefaultProfile("boss"),
			seccomp.WithDefaultProfile(),
		}
		if config.Network.Host {
			opts = append(opts, oci.WithHostHostsFile, oci.WithHostResolvconf, oci.WithHostNamespace(specs.NetworkNamespace))
		}
		logpath := filepath.Join(clix.GlobalString("log-path"), config.ID)
		container, err := client.NewContainer(
			ctx,
			config.ID,
			containerd.WithNewSpec(opts...),
			containerd.WithContainerLabels(map[string]string{
				"io.containerd/restart.status":  "running",
				"io.containerd/restart.logpath": logpath,
			}),
			containerd.WithNewSnapshot(config.ID, image),
		)
		if err != nil {
			return err
		}
		fmt.Printf("created container %s with logpath %s\n", config.ID, logpath)
		task, err := container.NewTask(ctx, cio.NullIO)
		if err != nil {
			return err
		}
		fmt.Println("starting container...")
		if err := task.Start(ctx); err != nil {
			return err
		}
		fmt.Printf("container %s started, have a great day!\n", config.ID)
		return nil
	},
}