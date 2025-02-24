package main

import (
	"fmt"
	"log"
	"net"
	"os"

	"github.com/miekg/dns"
	"gopkg.in/ini.v1" // Import the ini package for reading .ini files
	"gopkg.in/yaml.v2"
)

type Record struct {
	Name string `yaml:"name"`
	Type string `yaml:"type"`
	TTL  uint32 `yaml:"ttl"`
	Data string `yaml:"data"`
}

type Config struct {
	Records []Record `yaml:"records"`
}

var dnsRecords map[string][]dns.RR
var port string
var forwarder string
var queryLogging bool
var queryLogFile string
var queryLog *os.File

func loadConfig(filename string) error {
	cfg, err := ini.Load(filename)
	if err != nil {
		return err
	}

	// Load zone file
	zoneFile := cfg.Section("").Key("zone_file").String()
	if err := loadZoneData(zoneFile); err != nil {
		return err
	}

	// Load port
	port = cfg.Section("").Key("port").String()

	// Load forwarder
	forwarder = cfg.Section("").Key("forwarder").String()

	// Load query logging settings
	queryLogging = cfg.Section("").Key("query_logging").MustBool(false)
	queryLogFile = cfg.Section("").Key("query_log_file").String()

	// Log settings
	log.Printf("Configuration loaded:")
	log.Printf("  Zone file: %s", zoneFile)
	log.Printf("  Port: %s", port)
	log.Printf("  Forwarder: %s", forwarder)
	log.Printf("  Query logging: %t", queryLogging)
	log.Printf("  Query log file: %s", queryLogFile)

	// Open query log file if logging is enabled
	if queryLogging {
		var err error
		queryLog, err = os.OpenFile(queryLogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return fmt.Errorf("failed to open query log file: %v", err)
		}
	}

	return nil
}

func loadZoneData(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	var config Config
	if err := decoder.Decode(&config); err != nil {
		return err
	}

	dnsRecords = make(map[string][]dns.RR)
	for _, record := range config.Records {
		var rr dns.RR
		switch record.Type {
		case "A":
			rr = &dns.A{
				Hdr: dns.RR_Header{
					Name:   record.Name,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    record.TTL,
				},
				A: net.ParseIP(record.Data),
			}
		case "CNAME":
			rr = &dns.CNAME{
				Hdr: dns.RR_Header{
					Name:   record.Name,
					Rrtype: dns.TypeCNAME,
					Class:  dns.ClassINET,
					Ttl:    record.TTL,
				},
				Target: record.Data,
			}
		default:
			log.Printf("Unsupported record type: %s", record.Type)
			continue
		}
		dnsRecords[record.Name] = append(dnsRecords[record.Name], rr)
		log.Printf("Loaded record: %s %s %d %s", record.Name, record.Type, record.TTL, record.Data)
	}
	log.Printf("Zone file loaded successfully.")
	return nil
}

func logQuery(query string, responseType string) {
	if queryLogging && queryLog != nil {
		logLine := fmt.Sprintf("Query: %s, Response: %s\n", query, responseType)
		queryLog.WriteString(logLine)
	}
}

func handleRequest(w dns.ResponseWriter, r *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(r)

	for _, question := range r.Question {
		if records, ok := dnsRecords[question.Name]; ok {
			// If we have records for the requested name, respond with them
			m.Answer = append(m.Answer, records...)
			logQuery(question.Name, "Authoritative response")
		} else {
			// If no records found, forward to the upstream DNS server
			if forwarder != "" {
				c := new(dns.Client)
				resp, _, err := c.Exchange(r, forwarder+":53")
				if err == nil {
					// Forward the response from the upstream server
					w.WriteMsg(resp)
					logQuery(question.Name, "Forwarded response")
					return
				} else {
					log.Printf("Error forwarding request: %v", err)
				}
			}
			// If no records found and no forwarding occurred, respond with NXDOMAIN
			m.SetRcode(r, dns.RcodeNameError)
			logQuery(question.Name, "NXDOMAIN response")
		}
	}

	// Send the response back to the client
	w.WriteMsg(m)
}

func main() {
	// Load the configuration from the settings file
	if err := loadConfig("settings.conf"); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Set up the DNS handler for incoming requests
	dns.HandleFunc(".", handleRequest)

	// Create and start the DNS server
	server := &dns.Server{Addr: fmt.Sprintf(":%s", port), Net: "udp"}
	log.Printf("Starting DNS server on :%s\n", port)
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Failed to start server: %s\n", err.Error())
	}

	// Close the query log file if it was opened
	if queryLog != nil {
		queryLog.Close()
	}
}
