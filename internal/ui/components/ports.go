package components

const (
	portMin = 1024
	portMax = 65535
)

const (
	servicePortSSH    = "22"
	servicePortHTTP   = "80"
	servicePortHTTPS  = "443"
	servicePortRDP    = "3389"
	servicePortVNC    = "5900"
	servicePortCustom = "custom"
)

var servicePortNames = []string{
	"SSH",
	"HTTP",
	"HTTPS",
	"RDP",
	"VNC",
	"custom",
}

var servicePortMap = map[string]string{
	"SSH":   servicePortSSH,
	"HTTP":  servicePortHTTP,
	"HTTPS": servicePortHTTPS,
	"RDP":   servicePortRDP,
	"VNC":   servicePortVNC,
}
