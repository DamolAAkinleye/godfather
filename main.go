package main

import (
	"fmt"
	"github.com/miekg/dns"
	"log"
	"time"
)

func main() {
	zones := []string{"in.creditcards.com.", "staging.in.creditcards.com."}

	for _, zone := range zones {
		// ccads.in.creditcars.com - 10.11.100.30
		records, err := transferRecords(zone, "10.11.100.30")

		if err != nil {
			log.Fatalf("Error fetching records: %s\n", err)
		}

		if err := replicateRecords(records); err != nil {
			log.Printf("Error replicating zone %s: %s\n", zone, err)
		}
	}
}

// Transfer records from the dns zone `z` and nameserver `ns`
// returning an array of all resource records
func transferRecords(z string, ns string) ([]dns.RR, error) {
	tx := dns.Transfer{}
	msg := dns.Msg{}
	var records []dns.RR

	msg.SetAxfr(z)
	msg.SetTsig("axrf.", dns.HmacMD5, 300, time.Now().Unix())

	c, err := tx.In(&msg, ns+":53")

	if err != nil {
		return nil, err
	}

	for env := range c {
		if env.Error != nil {
			return records, nil
		}

		records = append(records, env.RR...)
	}

	return records, nil
}

func replicateRecords(rs []dns.RR) error {
	for _, record := range rs {
		switch t := interface{}(record).(type) {
		case *dns.A:
			fmt.Printf("A: \"%v\", \"%v\"\n", t.Hdr.Name, t.A)
		case *dns.CNAME:
			fmt.Printf("CNAME: \"%v\", \"%v\"\n", t.Hdr.Name, t.Target)
		case *dns.MX:
			fmt.Printf("MX: \"%v\", \"%v\"\n", t.Hdr.Name, t.Mx)
		default:
			// NOOP
		}
	}

	return nil
}
