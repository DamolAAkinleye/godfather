package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/miekg/dns"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/route53"
)

var name string
var target string
var TTL int64
var weight = int64(1)
var zoneId string

func init() {
	flag.StringVar(&name, "d", "", "domain name")
	flag.StringVar(&target, "t", "", "target of domain name")
	flag.StringVar(&zoneId, "z", "", "AWS Zone Id for domain")
	flag.Int64Var(&TTL, "ttl", int64(60), "TTL for DNS Cache")

}

fmt.Printf(name)
fmt.Printf(target)
fmt.Printf(TTL)
fmt.Printf(weight)
fmt.Printf(zoneId)


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
		case *dns.TXT:
			fmt.Printf("TXT: \"%v\", \"%v\"\n", t.Hdr.Name, t.Txt)
		default:
			// NOOP
		}
	}

	return nil
}
