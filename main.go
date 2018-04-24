package main

import (
	"fmt"
	"github.com/miekg/dns"
	"log"
	"time"
)

func main() {
	// 10.11.100.30
	records, err := transferRecords("in.creditcards.com.", "10.11.100.30")

	if err != nil {
		log.Fatalf("Error fetching records: %s\n", err)
	}

	for _, record := range records {
		switch t := interface{}(record).(type) {
		case *dns.A:
			fmt.Printf("A: \"%v\", \"%v\"\n", t.Hdr.Name, t.A)
		case *dns.CNAME:
			fmt.Printf("CNAME: \"%v\", \"%v\"\n", t.Hdr.Name, t.Target)
		default:
			// NOOP
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
