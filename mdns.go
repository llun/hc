package hc

import (
	"github.com/guelfey/go.dbus"

	"os"
	"strings"
)

// MDNSService represents a mDNS service.
type MDNSService struct {
	config *Config
}

// NewMDNSService returns a new service based for the bridge name, id and port.
func NewMDNSService(config *Config) *MDNSService {
	return &MDNSService{
		config: config,
	}
}

// Publish announces the service for the machine's ip address on a random port using mDNS.
func (s *MDNSService) Publish() error {
	hostname, _ := os.Hostname()
	stripped := strings.Replace(s.config.name, " ", "_", -1)
	var dconn *dbus.Conn
	var obj *dbus.Object
	var path dbus.ObjectPath
	var err error

	dconn, err = dbus.SystemBus()
	if err != nil {
		return err
	}

	obj = dconn.Object("org.freedesktop.Avahi", "/")
	obj.Call("org.freedesktop.Avahi.Server.EntryGroupNew", 0).Store(&path)

	obj = dconn.Object("org.freedesktop.Avahi", path)

	records := s.config.txtRecords()
	var text [][]byte = make([][]byte, len(records))
	for idx, record := range records {
		text[idx] = []byte(record)
	}

	// http://www.dns-sd.org/ServiceTypes.html
	obj.Call("org.freedesktop.Avahi.EntryGroup.AddService", 0,
		int32(-1),                  // avahi.IF_UNSPEC
		int32(-1),                  // avahi.PROTO_UNSPEC
		uint32(0),                  // flags
		stripped,                   // sname
		"_hap._tcp",                // stype
		"local",                    // sdomain
		hostname,                   // shost
		uint16(s.config.servePort), // port
		text) // text record
	obj.Call("org.freedesktop.Avahi.EntryGroup.Commit", 0)
	return nil
}
