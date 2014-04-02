package platform_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	boshlog "bosh/logger"
	. "bosh/platform"
	fakecd "bosh/platform/cdutil/fakes"
	boshcmd "bosh/platform/commands"
	fakedisk "bosh/platform/disk/fakes"
	boshnet "bosh/platform/net"
	fakestats "bosh/platform/stats/fakes"
	boshvitals "bosh/platform/vitals"
	boshsettings "bosh/settings"
	boshdirs "bosh/settings/directories"
	fakesys "bosh/system/fakes"
)

const UBUNTU_EXPECTED_NETWORK_INTERFACES = `auto lo
iface lo inet loopback

auto eth0
iface eth0 inet static
    address 192.168.195.6
    network 192.168.195.0
    netmask 255.255.255.0
    broadcast 192.168.195.255
    gateway 192.168.195.1`

const UBUNTU_EXPECTED_RESOLV_CONF = `nameserver 10.80.130.1
nameserver 10.80.130.2
`

func init() {
	const UBUNTU_EXPECTED_DHCP_CONFIG = `# Generated by bosh-agent

option rfc3442-classless-static-routes code 121 = array of unsigned integer 8;

send host-name "<hostname>";

request subnet-mask, broadcast-address, time-offset, routers,
	domain-name, domain-name-servers, domain-search, host-name,
	netbios-name-servers, netbios-scope, interface-mtu,
	rfc3442-classless-static-routes, ntp-servers;

prepend domain-name-servers zz.zz.zz.zz;
prepend domain-name-servers yy.yy.yy.yy;
prepend domain-name-servers xx.xx.xx.xx;
`

	Describe("Testing with Ginkgo", func() {
		var (
			collector       *fakestats.FakeStatsCollector
			fs              *fakesys.FakeFileSystem
			cmdRunner       *fakesys.FakeCmdRunner
			diskManager     *fakedisk.FakeDiskManager
			dirProvider     boshdirs.DirectoriesProvider
			diskWaitTimeout time.Duration
			platform        Platform
			cdutil          *fakecd.FakeCdUtil
			compressor      boshcmd.Compressor
			copier          boshcmd.Copier
			vitalsService   boshvitals.Service
			logger          boshlog.Logger
		)

		BeforeEach(func() {
			collector = &fakestats.FakeStatsCollector{}
			fs = fakesys.NewFakeFileSystem()
			cmdRunner = fakesys.NewFakeCmdRunner()
			diskManager = fakedisk.NewFakeDiskManager()
			dirProvider = boshdirs.NewDirectoriesProvider("/fake-dir")
			diskWaitTimeout = 1 * time.Millisecond
			cdutil = fakecd.NewFakeCdUtil()
			compressor = boshcmd.NewTarballCompressor(cmdRunner, fs)
			copier = boshcmd.NewCpCopier(cmdRunner, fs)
			vitalsService = boshvitals.NewService(collector, dirProvider)
			logger = boshlog.NewLogger(boshlog.LEVEL_NONE)
		})

		JustBeforeEach(func() {
			netManager := boshnet.NewUbuntuNetManager(fs, cmdRunner, 1*time.Millisecond)

			platform = NewLinuxPlatform(
				fs,
				cmdRunner,
				collector,
				compressor,
				copier,
				dirProvider,
				vitalsService,
				cdutil,
				diskManager,
				netManager,
				1*time.Millisecond,
				logger,
			)
		})

		Describe("SetupDhcp", func() {
			Context("when dhcp was not previously configured", func() {
				It("sets up dhcp and restarts dhclient", func() {
					networks := boshsettings.Networks{
						"bosh": boshsettings.Network{
							Default: []string{"dns"},
							Dns:     []string{"xx.xx.xx.xx", "yy.yy.yy.yy", "zz.zz.zz.zz"},
						},
						"vip": boshsettings.Network{
							Default: []string{},
							Dns:     []string{"aa.aa.aa.aa"},
						},
					}

					platform.SetupDhcp(networks)

					dhcpConfig := fs.GetFileTestStat("/etc/dhcp3/dhclient.conf")
					Expect(dhcpConfig).ToNot(BeNil())
					Expect(dhcpConfig.StringContents()).To(Equal(UBUNTU_EXPECTED_DHCP_CONFIG))

					Expect(len(cmdRunner.RunCommands)).To(Equal(2))
					Expect(cmdRunner.RunCommands[0]).To(Equal([]string{"pkill", "dhclient3"}))
					Expect(cmdRunner.RunCommands[1]).To(Equal([]string{"/etc/init.d/networking", "restart"}))
				})
			})

			Context("when dhcp was previously configured with different configuration", func() {
				It("sets up dhcp and restarts dhclient", func() {
					fs.WriteFileString("/etc/dhcp3/dhclient.conf", "fake-other-configuration")

					networks := boshsettings.Networks{
						"bosh": boshsettings.Network{
							Default: []string{"dns"},
							Dns:     []string{"xx.xx.xx.xx", "yy.yy.yy.yy", "zz.zz.zz.zz"},
						},
						"vip": boshsettings.Network{
							Default: []string{},
							Dns:     []string{"aa.aa.aa.aa"},
						},
					}

					platform.SetupDhcp(networks)

					dhcpConfig := fs.GetFileTestStat("/etc/dhcp3/dhclient.conf")
					Expect(dhcpConfig).ToNot(BeNil())
					Expect(dhcpConfig.StringContents()).To(Equal(UBUNTU_EXPECTED_DHCP_CONFIG))

					Expect(len(cmdRunner.RunCommands)).To(Equal(2))
					Expect(cmdRunner.RunCommands[0]).To(Equal([]string{"pkill", "dhclient3"}))
					Expect(cmdRunner.RunCommands[1]).To(Equal([]string{"/etc/init.d/networking", "restart"}))
				})
			})

			Context("when dhcp was previously configured with the same configuration", func() {
				It("does not restart dhclient", func() {
					fs.WriteFileString("/etc/dhcp3/dhclient.conf", UBUNTU_EXPECTED_DHCP_CONFIG)

					networks := boshsettings.Networks{
						"bosh": boshsettings.Network{
							Default: []string{"dns"},
							Dns:     []string{"xx.xx.xx.xx", "yy.yy.yy.yy", "zz.zz.zz.zz"},
						},
						"vip": boshsettings.Network{
							Default: []string{},
							Dns:     []string{"aa.aa.aa.aa"},
						},
					}

					platform.SetupDhcp(networks)

					dhcpConfig := fs.GetFileTestStat("/etc/dhcp3/dhclient.conf")
					Expect(dhcpConfig).ToNot(BeNil())
					Expect(dhcpConfig.StringContents()).To(Equal(UBUNTU_EXPECTED_DHCP_CONFIG))

					Expect(len(cmdRunner.RunCommands)).To(Equal(0))
				})
			})
		})

		Describe("SetupManualNetworking", func() {
			It("sets up interface and restarts networking", func() {
				networks := boshsettings.Networks{
					"bosh": boshsettings.Network{
						Default: []string{"dns", "gateway"},
						Ip:      "192.168.195.6",
						Netmask: "255.255.255.0",
						Gateway: "192.168.195.1",
						Mac:     "22:00:0a:1f:ac:2a",
						Dns:     []string{"10.80.130.2", "10.80.130.1"},
					},
				}
				fs.WriteFile("/sys/class/net/eth0", []byte{})
				fs.WriteFileString("/sys/class/net/eth0/address", "22:00:0a:1f:ac:2a\n")
				fs.SetGlob("/sys/class/net/*", []string{"/sys/class/net/eth0"})

				platform.SetupManualNetworking(networks)

				networkConfig := fs.GetFileTestStat("/etc/network/interfaces")
				Expect(networkConfig).ToNot(BeNil())
				Expect(networkConfig.StringContents()).To(Equal(UBUNTU_EXPECTED_NETWORK_INTERFACES))

				resolvConf := fs.GetFileTestStat("/etc/resolv.conf")
				Expect(resolvConf).ToNot(BeNil())
				Expect(resolvConf.StringContents()).To(Equal(UBUNTU_EXPECTED_RESOLV_CONF))

				time.Sleep(100 * time.Millisecond)

				Expect(len(cmdRunner.RunCommands)).To(Equal(8))
				Expect(cmdRunner.RunCommands[0]).To(Equal([]string{"service", "network-interface", "stop", "INTERFACE=eth0"}))
				Expect(cmdRunner.RunCommands[1]).To(Equal([]string{"service", "network-interface", "start", "INTERFACE=eth0"}))
				Expect(cmdRunner.RunCommands[2]).To(Equal([]string{"arping", "-c", "1", "-U", "-I", "eth0", "192.168.195.6"}))
				Expect(cmdRunner.RunCommands[7]).To(Equal([]string{"arping", "-c", "1", "-U", "-I", "eth0", "192.168.195.6"}))
			})
		})
	})
}
