package components

const (
	portMin = 1024
	portMax = 65535
)

const (
	serviceSSHPort   = "22"
	serviceHTTPPort  = "80"
	serviceHTTPSPort = "443"
	serviceRDPPort   = "3389"
	serviceVNCPort   = "5900"

	serviceSSHName    = "SSH"
	serviceHTTPName   = "HTTP"
	serviceHTTPSName  = "HTTPS"
	serviceRDPName    = "RDP"
	serviceVNCName    = "VNC"
	serviceCustomName = "custom"
)

var servicePortNames = []string{
	serviceSSHName,
	serviceHTTPName,
	serviceHTTPSName,
	serviceRDPName,
	serviceVNCName,
	serviceCustomName,
}

var servicePortMap = map[string]string{
	serviceSSHName:   serviceSSHPort,
	serviceHTTPName:  serviceHTTPPort,
	serviceHTTPSName: serviceHTTPSPort,
	serviceRDPName:   serviceRDPPort,
	serviceVNCName:   serviceVNCPort,
}
