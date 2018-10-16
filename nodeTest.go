package main

/* Al useful imports */
import (
	"errors"
	"flag"
	"fmt"
	"net"
)

/* The entry point for our System */
func main() {
	/* Parse the provided parameters on command line */
	// ethPrivateKey := flag.String("ethprivatekey", "af090175729a5437ff3fedb766a5c8dc8a4a783ed41a384b83cf4647f0c99824", "provide eth private key, defaults af090175729a5437ff3fedb766a5c8dc8a4a783ed41a384b83cf4647f0c99824")
	flag.Parse()

	conf := loadConfig()
	list := setUpEth(conf)

	/* Register Node */
	nodeIp, err := findExternalIP()
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("Node IP Address: " + nodeIp + ":" + string(conf.MyPort))
	// err = nodeListInstance.ListNode(nodeIp + ":" + string(conf.MyPort))
	// if err != nil {
	// 	fmt.Println(err)
	// }

	if len(list) != 0 {
		fmt.Println("Connecting to other nodes: ", len(list), " nodes....")

	} else {
		fmt.Println("No existing nodes to connect to")
	}
	test := make([]string, 1)
	test[0] = "http://localhost:" + string(conf.MyPort) + "/jrpc"
	go setUpClient(test)
	setUpServer(string(conf.MyPort))

}

func findExternalIP() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue // interface down
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue // loopback interface
		}
		addrs, err := iface.Addrs()
		if err != nil {
			return "", err
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ip = ip.To4()
			if ip == nil {
				continue // not an ipv4 address
			}
			return ip.String(), nil
		}
	}
	return "", errors.New("are you connected to the network?")
}
