package microvm

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/firecracker-microvm/firecracker-go-sdk"
	"github.com/firecracker-microvm/firecracker-go-sdk/client/models"
	"golang.org/x/crypto/ssh"
)

const (
	SOCKET_PLACEHOLDER = "$"
	SOCKET_PATH        = "/tmp/firecracker-" + SOCKET_PLACEHOLDER + ".socket"
	binPath            = "/app/firecracker"
	IP_PRE             = "172.16.0."
	MAC_PRE            = "06:00:AC:10:00:"
	BRIDGE             = "br0"
	RUNTIME_PATH       = "/usr/local/bin/go_js"
)

func generateConfig(staticConfiguration *firecracker.StaticNetworkConfiguration, socketPath string, ip string) firecracker.Config {
	kernelArgs := fmt.Sprintf("reboot=k panic=1 pci=off ip=%s::172.16.0.1:255.255.255.0::eth0:off", ip)
	return firecracker.Config{
		SocketPath:      socketPath,
		KernelImagePath: "/app/vmlinux",
		KernelArgs:      kernelArgs,
		MachineCfg: models.MachineConfiguration{
			VcpuCount:  firecracker.Int64(1),
			MemSizeMib: firecracker.Int64(512),
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
				StaticConfiguration: staticConfiguration,
			},
		},
	}
}

var ipPool chan int

func createIpAddress(vmId int) string {
	return fmt.Sprintf("%s%d", IP_PRE, vmId)
}

func createMACAddress(vmId int) string {
	return fmt.Sprintf("%s%02X", MAC_PRE, vmId)
}

func createTapStr(vmId int) string {
	return fmt.Sprintf("pizza-tap%d", vmId)
}

func createTap(tapName string) {
	err := exec.Command("ip", "tuntap", "add", "dev", tapName, "mode", "tap").Run()
	if err != nil {
		log.Fatalf("Fatal error %v", err)
	}
	err = exec.Command("ip", "link", "set", "dev", tapName, "master", BRIDGE).Run()
	if err != nil {
		log.Fatalf("Fatal error %v", err)
	}
	err = exec.Command("ip", "link", "set", "dev", tapName, "up").Run()
	if err != nil {
		log.Fatalf("Fatal error %v", err)
	}
}

func clearTap(tapName string) {
	err := exec.Command("ip", "link", "del", tapName).Run()
	if err != nil {
		log.Fatalf("Fatal error %v", err)
	}
}

func runSSHCommand(client *ssh.Client, cmd string) ([]byte, error) {
	session, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	return session.CombinedOutput(cmd)
}

func RunCode(src string, ctx context.Context) string {
	vmId := <-ipPool
	ip := createIpAddress(vmId)
	mac := createMACAddress(vmId)
	tap := createTapStr(vmId)
	socket := strings.Replace(SOCKET_PATH, SOCKET_PLACEHOLDER, strconv.Itoa(vmId), 1)

	createTap(tap)

	staticConfiguration := &firecracker.StaticNetworkConfiguration{
		MacAddress:  mac,
		HostDevName: tap,
	}

	config := generateConfig(staticConfiguration, socket, ip)

	cmd := firecracker.VMCommandBuilder{}.
		WithBin(binPath).
		WithSocketPath(socket).
		WithStderr(os.Stderr).
		Build(ctx)

	m, err := firecracker.NewMachine(ctx, config, firecracker.WithProcessRunner(cmd))
	if err != nil {
		log.Fatalf("Failed to initialize machine: %v", err)
	}

	if err := m.Start(ctx); err != nil {
		log.Fatalf("Failed to start machine: %v", err)
	}

	key, err := os.ReadFile("/app/id_rsa")
	if err != nil {
		log.Fatalf("Unable to read private key: %v", err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		log.Fatalf("Unable to parse private key: %v", err)
	}

	sshConfig := &ssh.ClientConfig{
		User: "root",
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         2 * time.Second,
	}

	var client *ssh.Client
	maxRetries := 20

	for i := range maxRetries {
		client, err = ssh.Dial("tcp", ip+":22", sshConfig)
		if err == nil {
			break
		}
		log.Printf("SSH not ready yet (attempt %d/%d). Retrying in 500ms...", i+1, maxRetries)
		time.Sleep(500 * time.Millisecond)
	}

	if err != nil {
		log.Fatalf("Failed to dial after %d attempts: %v", maxRetries, err)
	}
	defer client.Close()

	_, err = runSSHCommand(client, "mount -t tmpfs -o size=50M tmpfs /workspace")
	if err != nil {
		log.Fatalf("Failed to mount tmpfs: %v", err)
	}

	dst := make([]byte, base64.StdEncoding.EncodedLen(len(src)))
	base64.StdEncoding.Encode(dst, []byte(src))

	log.Println("Injecting encoded user code into microVM...")
	injectCmd := fmt.Sprintf("echo \"%s\" | base64 -d > /workspace/script.js", dst)

	_, err = runSSHCommand(client, injectCmd)
	if err != nil {
		log.Fatalf("Failed to inject user code to vm's tmpfs: %v", err)
	}

	runCmd := fmt.Sprintf("%s /workspace/script.js", RUNTIME_PATH)
	output, err := runSSHCommand(client, runCmd)
	if err != nil {
		log.Fatalf("Failed to run user script: %v\nOutput: %s", err, string(output))
	}

	err = m.StopVMM()
	if err != nil {
		log.Fatalf("Failed to stop VM: %v", err)
	}

	clearTap(tap)
	ipPool <- vmId
	os.Remove(socket)

	return string(output)
}

func Init() {
	ipPool = make(chan int, 100)
	for i := 2; i <= 101; i++ {
		ipPool <- i
	}
}
