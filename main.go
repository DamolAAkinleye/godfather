package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/miekg/dns"
)

var name string
var target string
var TTL int64
var weight = int64(1)
var zoneId string

func init() {
	flag.StringVar(&name, "aws_domain_name", "", "domain name")
	flag.StringVar(&target, "aws_target_domain", "", "target of domain name")
	flag.StringVar(&zoneId, "aws_zoneId", "", "AWS Zone Id for domain")
	flag.Int64Var(&TTL, "ttl", int64(60), "TTL for DNS Cache")

	fmt.Println(name)
	fmt.Println(target)
	fmt.Println(zoneId)
	fmt.Println(TTL)
	fmt.Println(weight)
}

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
	sess, err := session.NewSession()
	if err != nil {
		fmt.Println("failed to create session,", err)
		return
	}
	svc := route53.New(sess)

	listCNAMES(svc)

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

func listCNAMES(svc *route53.Route53) {
	// Now lets list all of the records.
	// For the life of me, I can't figure out how to get these lists to actually constrain the results.
	// AFAICT, supplying only the HostedZoneId returns exactly the same results as any valid input in all params.
	listParams := &route53.ListResourceRecordSetsInput{
		HostedZoneId: aws.String(zoneId), // Required
		// MaxItems:              aws.String("100"),
		// StartRecordIdentifier: aws.String("Sample update."),
		// StartRecordName:       aws.String("com."),
		// StartRecordType:       aws.String("CNAME"),
	}
	respList, err := svc.ListResourceRecordSets(listParams)

	if err != nil {
		// Print the error, cast err to awserr.Error to get the Code and
		// Message from an error.
		fmt.Println(err.Error())
		return
	}

	// Pretty-print the response data.
	fmt.Println("All records:")
	fmt.Println(respList)
}
