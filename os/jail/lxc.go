package jail

// Pre-requisites:
//    sudo apt-get install lxc bridge-utils

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	guidlib "github.com/vsekhar/govtil/guid"
	vexec "github.com/vsekhar/govtil/os/exec"
)

const VERSION_MAJOR = 0
const VERSION_MINOR = 1

const rawLxcTemplate = `
# hostname
lxc.utsname = {{.ID}}
#lxc.aa_profile = unconfined

# network configuration
lxc.network.type = veth
lxc.network.flags = up
lxc.network.link = {{.BridgeID}}
#lxc.network.link = lxcbr0
#lxc.network.name = eth0
lxc.network.mtu = 1500
lxc.network.hwaddr = 00:11:22:33:44:d
#lxc.network.ipv4 = 192.168.1.2/24
#lxc.network.ipv4 = 10.0.0.2/24
lxc.network.ipv4 = 0.0.0.0

# root filesystem
lxc.rootfs = {{.RootFS}}

# use a dedicated pts for the container (and limit the number of pseudo terminal
# available)
lxc.pts = 1024

# disable the main console
lxc.console = none

# no controlling tty at all
lxc.tty = 1

# no implicit access to devices
lxc.cgroup.devices.deny = a

# /dev/null and zero
lxc.cgroup.devices.allow = c 1:3 rwm
lxc.cgroup.devices.allow = c 1:5 rwm

# consoles
lxc.cgroup.devices.allow = c 5:1 rwm
lxc.cgroup.devices.allow = c 5:0 rwm
lxc.cgroup.devices.allow = c 4:0 rwm
lxc.cgroup.devices.allow = c 4:1 rwm

# /dev/urandom,/dev/random
lxc.cgroup.devices.allow = c 1:9 rwm
lxc.cgroup.devices.allow = c 1:8 rwm

# /dev/pts/* - pts namespaces are "coming soon"
lxc.cgroup.devices.allow = c 136:* rwm
lxc.cgroup.devices.allow = c 5:2 rwm

# tuntap
lxc.cgroup.devices.allow = c 10:200 rwm

# fuse
#lxc.cgroup.devices.allow = c 10:229 rwm

# rtc
#lxc.cgroup.devices.allow = c 254:0 rwm

# standard mount point
lxc.mount.entry = proc {{.RootFS}}/proc proc nosuid,nodev,noexec 0 0
lxc.mount.entry = sysfs {{.RootFS}}/sys sysfs nosuid,nodev,noexec 0 0
lxc.mount.entry = devpts {{.RootFS}}/dev/pts devpts newinstance,ptmxmode=0666,nosuid,noexec 0 0
#lxc.mount.entry = varrun {{.RootFS}}/var/run tmpfs mode=755,size=4096k,nosuid,nodev,noexec 0 0
#lxc.mount.entry = varlock {{.RootFS}}/var/lock tmpfs size=1024k,nosuid,nodev,noexec 0 0
#lxc.mount.entry = shm {{.RootFS}}/dev/shm tmpfs size=65536k,nosuid,nodev,noexec 0 0

# Inject docker-init
# lxc.mount.entry = [[.SysInitPath]] {{.RootFS}}/sbin/init none bind,ro 0 0

# Inject lxc-init
#lxc.mount.entry = /usr/lib/lxc/lxc-init {{.RootFS}}/usr/lib/lxc/lxc-init none bind,ro 0 0

# In order to get a working DNS environment, mount bind (ro) the host's /etc/resolv.conf into the container
# lxc.mount.entry = /etc/resolv.conf {{.RootFS}}/etc/resolv.conf none bind,ro 0 0

# drop linux capabilities (apply mainly to the user root in the container)
lxc.cap.drop = audit_control audit_write
# FIXME: "unknown capability block_suspend" ??
# lxc.cap.drop = block_suspend
# FIXME: "unknown capability block_suspend" ??
# lxc.cap.drop = wake_alarm
lxc.cap.drop = mac_admin mac_override
lxc.cap.drop = mknod
lxc.cap.drop = setfcap setpcap
lxc.cap.drop = sys_admin
lxc.cap.drop = sys_boot sys_module sys_nice sys_pacct sys_rawio
lxc.cap.drop = sys_resource sys_time sys_tty_config

# limits
{{if .Memory}}
lxc.cgroup.memory.limit_in_bytes = {{.Memory}}
lxc.cgroup.memory.soft_limit_in_bytes = {{.Memory}}
{{end}}
{{if .Swap}}
lxc.cgroup.memory.memsw.limit_in_bytes = {{.Swap}}
{{end}}
{{if .CPU}}
lxc.cgroup.cpu.shares = {{.CPU}}
{{end}}
`

const rawCmdTemplate = `
GJ_SUSPEND_HISTORY () {
	GJ_HISTCONTROL_OLD=
	if [[ ${HISTCONTROL-} ]]; then
		GJ_HC_SET=true
		GJ_HISTCONTROL_OLD=$HISTCONTROL
	else
		GJ_HC_SET=false
	fi
	HISTCONTROL=ignorespace
}

GJ_RESTORE_HISTORY() {
	if [[ $GJ_HC_SET ]]; then
		HISTCONTROL=$GJ_HISTCONTROL_OLD
	else
		unset HISTCONTROL 2> /dev/null || true
	fi
	unset GJ_HISTCONTROL_OLD 2> /dev/null || true
	unset GJ_HC_SET
}

GJ_SUSPEND_HISTORY
 set -o nounset
 set -o errexit

 # networking
 ip link set eth0 up
 ip addr add 10.0.0.2/24 dev eth0
 ip route add default via 10.0.0.1 dev eth0
 echo > /etc/resolv.conf nameserver 8.8.8.8
 echo >> /etc/resolv.conf nameserver 8.8.4.4
 echo >> /etc/resolv.conf search govtil_jail

 # Workaround: fix PWD
 cd {{.Pwd}}
GJ_RESTORE_HISTORY

# user command
{{.Cmd}}

GJ_SUSPEND_HISTORY
 # Write out env for next invocation
 mkdir -p {{.LibDir}}
 printenv > {{.LibDir}}/env
GJ_RESTORE_HISTORY
`

const MEG = 1024 * 1024

const ID_PREFIX = "govtil_jail_"
const LIB_DIR = "/var/lib/govtil_jail"
const extID = "wlan0" // 'real' external host network interface

var lxcTemplate *template.Template
var cmdTemplate *template.Template

func init() {
	var err error
	lxcTemplate, err = template.New("lxc").Parse(rawLxcTemplate)
	if err != nil {
		panic(err)
	}
	cmdTemplate, err = template.New("cmd").Parse(rawCmdTemplate)
	if err != nil {
		panic(err)
	}
}

type lxcjail struct {
	ID       string
	BridgeID string // bridge names have to be max 15 chars...
	RootFS   string
	Memory   int
	Swap     int
	CPU      int
	Ports    []uint
	Env      []string
}

func (l *lxcjail) Run(c *exec.Cmd) error {
	if len(c.Args) > 0 {
		return fmt.Errorf("Error, cannot have Args in cmd. Must be combined into a single Bash-parsable string in cmd.Path")
	}
	
	cmd := new(exec.Cmd)
	*cmd = *c
	// lxc-execute doesn't work because it wants to mount /proc and requires
	// CAP_SYS_ADMIN to do so, but we drop that capability in the conf file
	cmd.Path = "/usr/bin/lxc-start"
	cmd.Args = append([]string{
		cmd.Path,
		fmt.Sprintf("--name=%s", l.ID),
		fmt.Sprintf("--rcfile=%s", filepath.Join(os.TempDir(), l.ID)),
		"--", "bash", "-c",
	})
	if c.Env == nil {
		cmd.Env = l.Env
	} else {
		cmd.Env = c.Env
	}

	// Workaround: Lxc resets the working directory to root each time, so we
	// hunt down PWD here and set it manually in the above cmdScript
	pwd := "."
	if cmd.Env != nil {
		for _, s := range cmd.Env {
			parts := strings.SplitN(s, "=", 2)
			if len(parts) < 2 {
				continue
			}
			if parts[0] == "PWD" {
				trimmed := strings.TrimSpace(parts[1])
				if len(trimmed) > 0 {
					pwd = trimmed
				}
				break
			}
		}
	}

	// Prepare bash script and run
	scriptBuf := new(bytes.Buffer)
	type settings struct {
		Cmd string
		LibDir string
		Pwd string
	}
	s := settings{
		Cmd: c.Path,
		LibDir: LIB_DIR,
		Pwd: pwd,
	}
	if err := cmdTemplate.Execute(scriptBuf, s); err != nil {
		return err
	}
	cmd.Args = append(cmd.Args, scriptBuf.String())
	if err := cmd.Start(); err != nil {
		return err
	}
	
	// Forward Ctrl-C to underlying process, to allow Go to cleanup
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	go func() {
		for s := range sigCh {
			cmd.Process.Signal(s)
		}
	}()

	cmdErr := cmd.Wait()

	// Read and parse resulting env
	envBuf := new(bytes.Buffer)
	envFilePath := filepath.Join(l.RootFS, s.LibDir, "env")
	envFile, err := os.Open(envFilePath)
	if err != nil {
		return err
	}
	defer envFile.Close()
	envBuf.ReadFrom(envFile)
	l.Env = strings.Split(envBuf.String(), "\n")
	
	// We leave the env file as is on the filesystem. The "state" of the image
	// thus includes the final environment. This further enhances our identity
	// checking.

	return cmdErr
}

// Accumulate errors
func accErrs(acc *[]string, err error) {
	if err != nil {
		*acc = append(*acc, err.Error())
	}
}

// CleanUp reverses the steps done in NewLxcJail:
//
//  1. Delete configuration file
//  2. Remove iptables and NAT routing
//  3. Remove network bridge
func (l *lxcjail) CleanUp() error {
	errtext := make([]string, 0)

	// 1. Delete config file
	cfilename := filepath.Join(os.TempDir(), l.ID)
	accErrs(&errtext, os.Remove(cfilename))

	// 2. Remove iptables and NAT routing
	// TODO: remove NAT routing
	/*
		if l.Ports != nil {
			rports := make([]uint, len(l.Ports))
			copy(rports, l.Ports)
			for i, j := 0, len(rports)-1; i < j; i, j = i+1, j-1 {
				rports[i], rports[j] = rports[j], rports[i]
			}
		}
	*/

	accErrs(&errtext, iptables("-D", "FORWARD", "-i", l.BridgeID, "-o", extID,
		"-j", "ACCEPT"))
	accErrs(&errtext, iptables("-D", "FORWARD", "-i", extID, "-o", l.BridgeID,
		"-m", "state", "--state", "RELATED,ESTABLISHED", "-j", "ACCEPT"))
	accErrs(&errtext, iptables("-t", "nat", "-D", "POSTROUTING", "-j", "MASQUERADE", "-o", extID))
	accErrs(&errtext, iptables("-t", "nat", "-D", "OUTPUT", "-j", l.ID))
	accErrs(&errtext, iptables("-t", "nat", "-D", "PREROUTING", "-j", l.ID))
	accErrs(&errtext, iptables("-t", "nat", "-F", l.ID))
	accErrs(&errtext, iptables("-t", "nat", "-X", l.ID))

	// 3. Remove bridge
	accErrs(&errtext, ifconfig(l.BridgeID, "down"))
	accErrs(&errtext, brctl("delbr", l.BridgeID))

	if len(errtext) == 0 {
		return nil
	}
	return errors.New(strings.Join(errtext, "\n"))
}

// NewLxcJail creates an lxc-based jail at the specified path using the
// following steps:
//
//  1. Generate a unique ID (for naming resources)
//  2. Create a new network bridge
//  3. Setup iptables and NAT routing for requested ports
//  4. Generate and write lxc configuration file
//
// This jail's multi-threaded behavior is not yet known.
func NewLxcJail(root string, ports []uint, env []string) (Interface, error) {
	l := new(lxcjail)

	// 1. Unique ID
	guid, err := guidlib.V4()
	if err != nil {
		return nil, err
	}
	l.ID = ID_PREFIX + guid.Short()
	l.RootFS = root
	l.Memory = 512 * MEG
	// leave defaults for l.Swap and l.CPU

	// 2. Create network bridge
	l.BridgeID = "gj_" + guid.Short() // max 15 chars
	if err := brctl("addbr", l.BridgeID); err != nil {
		return nil, err
	}
	if err := ifconfig(l.BridgeID, "10.0.0.1", "netmask", "255.0.0.0", "up"); err != nil {
		return nil, err
	}

	// 3. Setup iptables...
	if err := iptables("-t", "nat", "-N", l.ID); err != nil {
		return nil, fmt.Errorf("Failed to create %s chain: %s", l.ID, err)
	}
	if err := iptables("-t", "nat", "-A", "PREROUTING", "-j", l.ID); err != nil {
		return nil, fmt.Errorf("Failed to inject %s in PREROUTING chain: %s", l.ID, err)
	}
	if err := iptables("-t", "nat", "-A", "OUTPUT", "-j", l.ID); err != nil {
		return nil, fmt.Errorf("Failed to inject %s in OUTPUT chain: %s", l.ID, err)
	}

	// ... IP re-write
	if err := iptables("-t", "nat", "-A", "POSTROUTING", "-j", "MASQUERADE", "-o", extID); err != nil {
		return nil, fmt.Errorf("Failed to enable MASQUERADE in POSTROUTING chain: %s", err)
	}
	// ... inbound forwarding (filtered)
	if err := iptables("-A", "FORWARD", "-i", extID, "-o", l.BridgeID,
		"-m", "state", "--state", "RELATED,ESTABLISHED", "-j", "ACCEPT"); err != nil {
		return nil, fmt.Errorf("Failed to enable FORWARD from %s to %s: %s", extID, l.BridgeID, err)
	}
	// ... output forwarding
	if err := iptables("-A", "FORWARD", "-i", l.BridgeID, "-o", extID,
		"-j", "ACCEPT"); err != nil {
		return nil, fmt.Errorf("Failed to enable FORWARD from %s to %s: %s", l.BridgeID, extID, err)
	}

	// TODO: Port forwarding
	/*
		l.Ports = ports
		if l.Ports != nil {
			for port := range l.Ports {
				if err:= iptables("-t", "nat", "-A", "l.ID", "-p", "tcp", "--dport",
					strconv.Itoa(port), "-j", "DNAT", "--to-destination",
					net.JoinHostPort(dest.IP.String(), strconv.Itoa(dest.Port))); err != nil {
					return nil, err
				}
			}
		}
	*/

	// 4. Generate configuration file
	cfilename := filepath.Join(os.TempDir(), l.ID)
	cfile, err := os.OpenFile(cfilename, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, err
	}
	defer cfile.Close()
	err = lxcTemplate.Execute(cfile, l)

	if env != nil {
		l.Env = env
	} else {
		l.Env = []string{
			"SHELL=/bin/bash",
			"USER=root",
			"LOGNAME=root",
			"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
			"PWD=/root",
			"HOME=/root",
			"GOVTIL_JAIL_LXC=true",
			fmt.Sprintf("GOVTIL_JAIL_LXC_MAJOR_VERSION=%d", VERSION_MAJOR),
			fmt.Sprintf("GOVTIL_JAIL_LXC_MINOR_VERSION=%d", VERSION_MINOR),
		}
	}

	return l, nil
}

// Utility functions

// Return the IPv4 address of a network interface
func getIfaceAddr(name string) (net.Addr, error) {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return nil, err
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return nil, err
	}
	var addrs4 []net.Addr
	for _, addr := range addrs {
		ip := (addr.(*net.IPNet)).IP
		if ip4 := ip.To4(); len(ip4) == net.IPv4len {
			addrs4 = append(addrs4, addr)
		}
	}
	switch {
	case len(addrs4) == 0:
		return nil, fmt.Errorf("Interface %v has no IP addresses", name)
	case len(addrs4) > 1:
		fmt.Printf("Interface %v has more than 1 IPv4 address. Defaulting to using %v\n",
			name, (addrs4[0].(*net.IPNet)).IP)
	}
	return addrs4[0], nil
}

// TODO: re-do for ip * commands rather than iptables, route, etc.
// see: http://dougvitale.wordpress.com/2011/12/21/deprecated-linux-networking-commands-and-their-replacements/#route

func iptables(args ...string) error { return vexec.Run("iptables", args...) }
func iptablesForward(rule string, chain string, port int, dest net.TCPAddr) error {
	return iptables(
		"-t", "nat", rule, chain,
		"-p", "tcp",
		"--dport", strconv.Itoa(port),
		"-j", "DNAT",
		"--to-destination", net.JoinHostPort(dest.IP.String(), strconv.Itoa(dest.Port)),
	)
}

func brctl(args ...string) error    { return vexec.Run("brctl", args...) }
func ifconfig(args ...string) error { return vexec.Run("ifconfig", args...) }
