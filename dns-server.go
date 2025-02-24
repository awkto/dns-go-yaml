package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

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
var enableForwarding bool
var zoneFileFormat string

func loadConfig(filename string) error {
	cfg, err := ini.Load(filename)
	if err != nil {
		return err
	}

	// Load zone file settings
	zoneFile := cfg.Section("").Key("zone_file").String()
	zoneFileFormat = cfg.Section("").Key("zone_file_format").String()

	// Verify file extension matches the specified format
	ext := filepath.Ext(zoneFile)
	switch zoneFileFormat {
	case "yaml":
		if ext != ".yaml" && ext != ".yml" {
			log.Fatalf("Zone file %s has extension %s, but format is specified as YAML", zoneFile, ext)
			return fmt.Errorf("zone file extension does not match specified format")
		}
	case "csv":
		if ext != ".csv" {
			log.Fatalf("Zone file %s has extension %s, but format is specified as CSV", zoneFile, ext)
			return fmt.Errorf("zone file extension does not match specified format")
		}
	}

	// Load zone data with the correct file and format
	if err := loadZoneData(zoneFile, zoneFileFormat); err != nil {
		return err
	}

	// Load port
	port = cfg.Section("").Key("port").String()

	// Load forwarder
	forwarder = cfg.Section("").Key("forwarder").String()

	// Load query logging settings
	queryLogging = cfg.Section("").Key("query_logging").MustBool(false)
	queryLogFile = cfg.Section("").Key("query_log_file").String()

	// Load enable forwarding setting
	enableForwarding = cfg.Section("").Key("enable_forwarding").MustBool(true)

	// Log settings
	log.Printf("Configuration loaded:")
	log.Printf("  Zone file: %s", zoneFile)
	log.Printf("  Port: %s", port)
	log.Printf("  Forwarder: %s", forwarder)
	log.Printf("  Query logging: %t", queryLogging)
	log.Printf("  Query log file: %s", queryLogFile)
	log.Printf("  Enable forwarding: %t", enableForwarding)

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

func loadZoneData(filename, format string) error {
	log.Printf("Loading zone data from file: %s with format: %s", filename, format) // Debug log

	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("error opening file %s: %w", filename, err)
	}
	defer file.Close()

	dnsRecords = make(map[string][]dns.RR)

	switch format {
	case "yaml":
		decoder := yaml.NewDecoder(file)
		var config Config
		err = decoder.Decode(&config)
		if err != nil {
			return fmt.Errorf("error decoding YAML file %s: %w", filename, err)
		}
		for _, record := range config.Records {
			addRecord(record)
		}
	case "csv":
		reader := csv.NewReader(file)
		reader.TrimLeadingSpace = true
		reader.LazyQuotes = true

		records, err := reader.ReadAll()
		if err != nil {
			return fmt.Errorf("error reading CSV file %s: %w", filename, err)
		}
		if len(records) <= 1 {
			log.Println("CSV file is empty or has only header")
			return nil
		}

		// Skip header row
		for i, record := range records[1:] {
			if len(record) != 4 {
				log.Printf("Invalid record format at line %d: %v", i+2, record) // i+2 because we skipped the header
				continue
			}
			ttl, err := parseTTL(record[2])
			if err != nil {
				log.Printf("Invalid TTL value at line %d: %v, error: %v", i+2, record[2], err)
				continue
			}
			recordData := Record{
				Name: record[0],
				Type: record[1],
				TTL:  ttl,
				Data: record[3],
			}
			addRecord(recordData)
		}

	default:
		return fmt.Errorf("unsupported zone file format: %s", format)
	}

	log.Printf("Zone file loaded successfully.")
	return nil
}

func parseTTL(ttlStr string) (uint32, error) {
	ttlInt, err := strconv.ParseUint(ttlStr, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("error parsing TTL: %w", err)
	}
	return uint32(ttlInt), nil
}

func addRecord(record Record) {
	// Normalize the record name by adding a trailing dot if it's missing
	if !strings.HasSuffix(record.Name, ".") {
		record.Name += "."
	}

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
		return
	}
	dnsRecords[strings.ToLower(record.Name)] = append(dnsRecords[strings.ToLower(record.Name)], rr)
	log.Printf("Loaded record: %s %s %d %s", record.Name, record.Type, record.TTL, record.Data)
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
		queryName := question.Name
		// Normalize the query name by adding a trailing dot if it's missing
		if !strings.HasSuffix(queryName, ".") {
			queryName += "."
		}

		queryName = strings.ToLower(queryName) // Convert query name to lowercase
		log.Printf("Received query for: %s", question.Name)

		records, ok := dnsRecords[queryName] // Use lowercase query name
		if ok {
			log.Printf("Found local records for %s", question.Name)
			if len(records) > 0 {
				log.Printf("Responding with local records")
				m.Answer = append(m.Answer, records...)
				logQuery(question.Name, "Authoritative response")
			} else {
				log.Printf("No records found for %s, but key exists", question.Name)
			}
		} else {
			log.Printf("No local records found for %s", question.Name)
			// If no records found, forward to the upstream DNS server if forwarding is enabled
			if enableForwarding && forwarder != "" {
				c := new(dns.Client)
				resp, _, err := c.Exchange(r, forwarder+":53")
				if err == nil {
					// Forward the response from the upstream server
					resp.CopyTo(m) // Copy the response to the message
					m.SetReply(r)  // Ensure the message is a reply
					w.WriteMsg(m)
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
