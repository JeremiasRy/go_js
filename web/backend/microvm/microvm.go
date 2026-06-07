package microvm

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/firecracker-microvm/firecracker-go-sdk"
	"github.com/firecracker-microvm/firecracker-go-sdk/client/models"
)

const (
	socketPath = "/tmp/firecracker.socket"
	binPath    = "/app/firecracker"
)

var config = firecracker.Config{
	SocketPath:      socketPath,
	KernelImagePath: "/app/vmlinux",
	MachineCfg: models.MachineConfiguration{
		VcpuCount:  firecracker.Int64(1),
		MemSizeMib: firecracker.Int64(128),
	},
	Drives: []models.Drive{
		{
			DriveID:      new("rootfs"),
			PathOnHost:   new("/app/rootfs.ext4"),
			IsRootDevice: new(true),
			IsReadOnly:   new(true),
		},
	},
	NetworkInterfaces: []firecracker.NetworkInterface{
		{
			StaticConfiguration: &firecracker.StaticNetworkConfiguration{
				HostDevName: "tap0",
				MacAddress:  "06:00:AC:10:00:02",
			},
		},
	},
}

func spinUpAMachine(ctx context.Context) *firecracker.Machine {
	cmd := firecracker.VMCommandBuilder{}.
		WithBin(binPath).
		WithSocketPath(socketPath).
		WithStderr(os.Stderr).
		Build(ctx)

	m, err := firecracker.NewMachine(ctx, config, firecracker.WithProcessRunner(cmd))
	if err != nil {
		log.Fatalf("Failed to initialize machine: %v", err)
	}

	fmt.Println("Starting Firecracker MicroVM...")
	if err := m.Start(ctx); err != nil {
		log.Fatalf("Failed to start machine: %v", err)
	}

	fmt.Println("MicroVM is successfully running!")

	if err := m.Wait(ctx); err != nil {
		log.Fatalf("Machine unexpectedly exited: %v", err)
	}

	return m
}

func Init() {
	os.Remove(socketPath)
}
