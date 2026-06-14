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
	log.Println("Waiting to acquire VM ID from pool...")
	vmId := <-ipPool
	log.Printf("Acquired VM ID: %v", vmId)

	log.Println("Generating network configuration (IP, MAC, TAP)...")
	ip := createIpAddress(vmId)
	mac := createMACAddress(vmId)
	tap := createTapStr(vmId)
	socket := strings.Replace(SOCKET_PATH, SOCKET_PLACEHOLDER, strconv.Itoa(vmId), 1)

	log.Printf("Creating TAP interface: %s", tap)
	createTap(tap)

	log.Println("Building Firecracker static network configuration...")
	staticConfiguration := &firecracker.StaticNetworkConfiguration{
		MacAddress:  mac,
		HostDevName: tap,
	}

	config := generateConfig(staticConfiguration, socket, ip)

	log.Println("Building VM command...")
	cmd := firecracker.VMCommandBuilder{}.
		WithBin(binPath).
		WithSocketPath(socket).
		WithStdout(os.Stdout).
		WithStderr(os.Stderr).
		Build(ctx)

	log.Println("Initializing Firecracker machine...")
	m, err := firecracker.NewMachine(ctx, config, firecracker.WithProcessRunner(cmd))
	if err != nil {
		log.Fatalf("Failed to initialize machine: %v", err)
	}

	log.Println("Starting Firecracker MicroVM...")
	if err := m.Start(ctx); err != nil {
		log.Fatalf("Failed to start machine: %v", err)
	}
	log.Println("MicroVM started successfully.")

	log.Println("Reading SSH private key...")
	key, err := os.ReadFile("/app/id_rsa")
	if err != nil {
		log.Fatalf("Unable to read private key: %v", err)
	}

	log.Println("Parsing SSH private key...")
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

	log.Printf("Connecting to microVM via SSH at %s:22...", ip)
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
	log.Println("SSH connection established.")

	log.Println("Mounting tmpfs workspace in microVM...")
	_, err = runSSHCommand(client, "mount -t tmpfs -o size=50M tmpfs /workspace")
	if err != nil {
		log.Fatalf("Failed to mount tmpfs: %v", err)
	}

	log.Println("Encoding user source code (Base64)...")
	dst := make([]byte, base64.StdEncoding.EncodedLen(len(src)))
	base64.StdEncoding.Encode(dst, []byte(src))

	log.Println("Injecting encoded user code into microVM...")
	injectCmd := fmt.Sprintf("echo \"%s\" | base64 -d > /workspace/script.js", dst)
	_, err = runSSHCommand(client, injectCmd)
	if err != nil {
		log.Fatalf("Failed to inject user code to vm's tmpfs: %v", err)
	}

	log.Println("Executing user script...")
	runCmd := fmt.Sprintf("%s /workspace/script.js", RUNTIME_PATH)
	output, err := runSSHCommand(client, runCmd)
	if err != nil {
		log.Fatalf("Failed to run user script: %v\nOutput: %s", err, string(output))
	}
	log.Println("User script executed successfully.")

	log.Println("Stopping Firecracker MicroVM...")
	err = m.StopVMM()
	if err != nil {
		log.Fatalf("Failed to stop VM: %v", err)
	}

	log.Printf("Clearing TAP interface: %s", tap)
	clearTap(tap)

	log.Printf("Returning VM ID %v to pool...", vmId)
	ipPool <- vmId
	os.Remove(socket)

	log.Println("Execution completed. Returning output.")
	return string(output)
}

func Init() {
	ipPool = make(chan int, 100)
	for i := 2; i <= 101; i++ {
		ipPool <- i
	}
}
