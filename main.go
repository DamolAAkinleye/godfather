package main

import (
	"flag"
	"fmt"
	"log"
	"strings"
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
	flag.StringVar(&name, "d", "", "domain name")
	flag.StringVar(&target, "t", "", "target of domain name")
	flag.StringVar(&zoneId, "z", "", "AWS Zone Id for domain")
	//flag.Int64Var(&TTL, "ttl", int64(60), "TTL for DNS Cache")
}

func main() {
	// Establish AWS session
	sess, err := session.NewSession()
	if err != nil {
		fmt.Println("failed to create session,", err)
		return
	}

	svc := route53.New(sess)

	//  HERP DERP DERP DERP DREP .
	flag.Parse()

	//Active Directory zones being pulled
	zones := []string{"in.creditcards.com."}

	for _, zone := range zones {
		// ccads.in.creditcars.com - 10.11.100.30
		records, err := transferRecords(zone, "10.11.100.30")

		if err != nil {
			log.Fatalf("Error fetching records: %s\n", err)
		}

		fmt.Printf("Total record count: %s\n", len(records))

		if err := replicateRecords(svc, records); err != nil {
			log.Printf("Error replicating zone %s: %s\n", zone, err)
		}
	}
	//End AD zone pull

	// Start Route53

	if name == "" || target == "" || zoneId == "" {
		fmt.Println(fmt.Errorf("Incomplete arguments: d: %s, t: %s, z: %s\n", name, target, zoneId))
		flag.PrintDefaults()
		return
	}

	//listCNAMES(svc)

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

// TODO: This requires quite a bit of cleanup
func replicateRecords(svc *route53.Route53, rs []dns.RR) error {
	cs := &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: aws.String(zoneId),
	}
	cb := &route53.ChangeBatch{
		Comment: aws.String(time.Now().UTC().String()),
	}
	var changes []*route53.Change

	for _, record := range rs {
		rrs := &route53.ResourceRecordSet{
			TTL:  aws.Int64(60),
			Name: aws.String(record.Header().Name),
		}

		switch t := interface{}(record).(type) {
		case *dns.A:
			rrs.SetType("A")
			rrs.ResourceRecords = []*route53.ResourceRecord{
				{
					Value: aws.String(t.A.String()),
				},
			}
		case *dns.CNAME:
			rrs.SetType("CNAME")
			rrs.ResourceRecords = []*route53.ResourceRecord{
				{
					Value: aws.String(t.Target),
				},
			}
		case *dns.MX:
			rrs.SetType("MX")
			rrs.ResourceRecords = []*route53.ResourceRecord{
				{
					Value: aws.String(t.Mx),
				},
			}
		case *dns.TXT:
			value := fmt.Sprintf("\"%s\"", strings.Join(t.Txt, " "))

			rrs.SetType("TXT")
			rrs.ResourceRecords = []*route53.ResourceRecord{
				{
					Value: aws.String(value),
				},
			}
		default:
			// NOOP
			continue
		}

		c := &route53.Change{
			Action:            aws.String("UPSERT"),
			ResourceRecordSet: rrs,
		}

		// Print out any validation errors
		if err := rrs.Validate(); err != nil {
			fmt.Printf("Invalid record: %s, %s", record.Header().Name, err)
		} else {
			if l := len(changes); l > 0 && *changes[l-1].ResourceRecordSet.Name == record.Header().Name {
				previous := changes[l-1].ResourceRecordSet.ResourceRecords

				previous = append(previous, rrs.ResourceRecords...)
			} else {
				changes = append(changes, c)
			}
		}

	}

	// TODO: Turn this into a loop on 500 record chunks
	chunkSize := 500
	for chunk := 0; (chunk * chunkSize) < len(changes); chunk++ {
		var bounds int

		if next := chunk*chunkSize + chunkSize; next < len(changes) {
			bounds = next
		} else {
			bounds = len(changes)
		}

		cb.SetChanges(changes[chunk*chunkSize : bounds])
		cs.SetChangeBatch(cb)

		resp, err := svc.ChangeResourceRecordSets(cs)

		if err != nil {
			return err
		}

		fmt.Printf("%#v\n", resp)
	}

	return nil
}

func listCNAMES(svc *route53.Route53) {
	// Now lets list all of the records.
	// For the life of me, I can't figure out how to get these lists to actually constrain the results.
	// AFAICT, supplying only the HostedZoneId returns exactly the same results as any valid input in all params.
	listParams := &route53.ListResourceRecordSetsInput{
		HostedZoneId: aws.String(zoneId), // Required
		// Test static zone below
		//HostedZoneId: aws.String("Z3BWSUB0RPS89Q"), // Required

		MaxItems: aws.String("1000"),
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
