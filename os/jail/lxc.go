package jail

// Pre-requisites:
//    sudo apt-get install lxc bridge-utils

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	guidlib "github.com/vsekhar/govtil/guid"
)

const raw_lxc_template = `
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

const cmdTemplate = `
#ifconfig eth0 10.0.0.2 netmask 255.0.0.0 up
#route add -host 10.0.0.1 dev eth0
#route add default gw 10.0.0.1 eth0
ip link set eth0 up
ip addr add 10.0.0.2/24 dev eth0
ip route add default via 10.0.0.1 dev eth0
#hostname; ifconfig; ip route show
echo > /etc/resolv.conf nameserver 8.8.8.8
echo >> /etc/resolv.conf nameserver 8.8.4.4
echo >> /etc/resolv.conf search govtil_jail
# user command on next line
%s
`

const MEG = 1024 * 1024

const ID_PREFIX = "govtil_jail_"
const extID = "wlan0" // 'real' external host network interface

var lxcTemplate *template.Template

func init() {
	// Compile lxc.conf template (use with template.Execute(out, config))
	var err error
	lxcTemplate, err = template.New("lxc").Parse(raw_lxc_template)
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
}

func (l *lxcjail) Imprison(c *exec.Cmd) (*exec.Cmd, error) {
	if len(c.Args) > 0 {
		return nil, fmt.Errorf("Error, cannot have Args in cmd. Must be combined into a single Bash-parsable string in cmd.Path")
	}
	user_cmd := c.Path
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
	script := fmt.Sprintf(cmdTemplate, user_cmd)
	cmd.Args = append(cmd.Args, script)

	return cmd, nil
}

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
func NewLxcJail(root string, ports []uint) (Interface, error) {
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

func iptables(args ...string) error { return cmd("iptables", args...) }
func iptablesForward(rule string, chain string, port int, dest net.TCPAddr) error {
	return iptables(
		"-t", "nat", rule, chain,
		"-p", "tcp",
		"--dport", strconv.Itoa(port),
		"-j", "DNAT",
		"--to-destination", net.JoinHostPort(dest.IP.String(), strconv.Itoa(dest.Port)),
	)
}

func brctl(args ...string) error    { return cmd("brctl", args...) }
func ifconfig(args ...string) error { return cmd("ifconfig", args...) }

// command wrapper
func cmd(cmd string, args ...string) error {
	path, err := exec.LookPath(cmd)
	if err != nil {
		return fmt.Errorf("command not found: %s", cmd)
	}
	if err := exec.Command(path, args...).Run(); err != nil {
		return fmt.Errorf("%s failed: %s %v", cmd, cmd, strings.Join(args, " "))
	}
	return nil
}
